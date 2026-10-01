const express = require('express');
const router = express.Router();
const { pgPool } = require('../config/db');
const { sendOrderConfirmation } = require('../services/emailService');

const {
  MPESA_CONSUMER_KEY,
  MPESA_CONSUMER_SECRET,
  MPESA_SHORTCODE,
  MPESA_PASSKEY,
  MPESA_BASE_URL = 'https://api.safaricom.co.ke',
  MPESA_CALLBACK_URL,
} = process.env;

// ─── Generate OAuth Access Token ─────────────────────────────────────────────
async function getAccessToken() {
  const credentials = Buffer.from(`${MPESA_CONSUMER_KEY}:${MPESA_CONSUMER_SECRET}`).toString('base64');
  const res = await fetch(`${MPESA_BASE_URL}/oauth/v1/generate?grant_type=client_credentials`, {
    headers: { Authorization: `Basic ${credentials}` },
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`Daraja OAuth failed: ${text}`);
  }
  const data = await res.json();
  return data.access_token;
}

// ─── Build Timestamp and Password ────────────────────────────────────────────
function getTimestampAndPassword() {
  const now = new Date();
  const pad = (n) => String(n).padStart(2, '0');
  const timestamp =
    `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}` +
    `${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`;
  const password = Buffer.from(`${MPESA_SHORTCODE}${MPESA_PASSKEY}${timestamp}`).toString('base64');
  return { timestamp, password };
}

// ─── POST /api/mpesa/stkpush ──────────────────────────────────────────────────
// Triggers the Lipa Na M-Pesa STK push prompt on the customer's phone.
// Body: { phone, amount, orderId, orderRef }
router.post('/stkpush', async (req, res) => {
  try {
    const { phone, amount, orderId, orderRef } = req.body;

    if (!phone || !amount) {
      return res.status(400).json({ message: 'Phone and amount are required' });
    }

    // Normalize phone to 254XXXXXXXXX format
    let normalizedPhone = phone.replace(/\D/g, '');
    if (normalizedPhone.startsWith('0')) normalizedPhone = '254' + normalizedPhone.slice(1);
    if (normalizedPhone.startsWith('+')) normalizedPhone = normalizedPhone.slice(1);

    const token = await getAccessToken();
    const { timestamp, password } = getTimestampAndPassword();

    const payload = {
      BusinessShortCode: MPESA_SHORTCODE,
      Password: password,
      Timestamp: timestamp,
      TransactionType: 'CustomerBuyGoodsOnline',
      Amount: Math.ceil(amount), // must be whole number
      PartyA: normalizedPhone,
      PartyB: MPESA_SHORTCODE,
      PhoneNumber: normalizedPhone,
      CallBackURL: MPESA_CALLBACK_URL,
      AccountReference: orderRef || 'NaojaVentures',
      TransactionDesc: `Payment for order ${orderRef || orderId}`,
    };

    const stkRes = await fetch(`${MPESA_BASE_URL}/mpesa/stkpush/v1/processrequest`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
    });

    const stkData = await stkRes.json();

    if (stkData.ResponseCode !== '0') {
      return res.status(400).json({ message: stkData.CustomerMessage || 'STK Push failed', raw: stkData });
    }

    // Associate checkoutRequestId with the order in PostgreSQL
    if (orderRef || orderId) {
      try {
        await pgPool.query(
          `UPDATE orders 
           SET mpesa_checkout_request_id = $1 
           WHERE order_ref = $2 OR id = $3`,
          [stkData.CheckoutRequestID, orderRef || '', parseInt(orderId, 10) || 0]
        );
      } catch (dbErr) {
        console.warn('⚠️ Could not attach checkoutRequestId to order in DB:', dbErr.message);
      }
    }

    res.json({
      success: true,
      message: 'STK Push sent! Ask the customer to check their phone.',
      checkoutRequestId: stkData.CheckoutRequestID,
    });
  } catch (err) {
    console.error('STK Push error:', err.message);
    res.status(500).json({ message: err.message });
  }
});

// ─── POST /api/mpesa/callback ─────────────────────────────────────────────────
// Safaricom will POST payment results here automatically.
router.post('/callback', async (req, res) => {
  try {
    const body = req.body;
    const stkCallback = body?.Body?.stkCallback;

    if (!stkCallback) {
      return res.status(400).json({ message: 'Invalid callback payload' });
    }

    const resultCode = stkCallback.ResultCode;
    const resultDesc = stkCallback.ResultDesc;
    const checkoutRequestId = stkCallback.CheckoutRequestID;

    console.log(`📲 M-Pesa Callback — ResultCode: ${resultCode} | ${resultDesc} | ReqID: ${checkoutRequestId}`);

    if (resultCode === 0) {
      // Payment was successful
      const metadata = stkCallback.CallbackMetadata?.Item || [];
      const getMeta = (name) => metadata.find((i) => i.Name === name)?.Value;

      const mpesaCode = getMeta('MpesaReceiptNumber');
      const phoneUsed = getMeta('PhoneNumber');
      const amountPaid = getMeta('Amount');

      console.log(`✅ Payment successful — Code: ${mpesaCode}, Amount: ${amountPaid}, Phone: ${phoneUsed}`);

      if (mpesaCode) {
        // 1. Primary match: Match order by unique CheckoutRequestID
        let updated = false;
        if (checkoutRequestId) {
          const result = await pgPool.query(
            `UPDATE orders 
             SET payment_status = 'paid', 
                 mpesa_transaction_id = $1,
                 status = CASE WHEN status = 'pending' THEN 'confirmed' ELSE status END
             WHERE mpesa_checkout_request_id = $2
             RETURNING *`,
            [mpesaCode, checkoutRequestId]
          );
          if (result.rowCount > 0) {
            updated = true;
            console.log(`🎯 Order ${result.rows[0].order_ref} marked paid via CheckoutRequestID.`);
            sendOrderConfirmation(result.rows[0]).catch(e => console.warn('Paid email error:', e.message));
          }
        }

        // 2. Secondary fallback: Match most recent pending order for that phone
        if (!updated && phoneUsed) {
          const fallbackResult = await pgPool.query(
            `UPDATE orders 
             SET payment_status = 'paid', 
                 mpesa_transaction_id = $1,
                 status = CASE WHEN status = 'pending' THEN 'confirmed' ELSE status END
             WHERE payment_status = 'pending'
               AND customer_phone LIKE $2
             ORDER BY created_at DESC
             LIMIT 1
             RETURNING *`,
            [mpesaCode, `%${String(phoneUsed).slice(-9)}`]
          );
          if (fallbackResult.rowCount > 0) {
            console.log(`🎯 Order ${fallbackResult.rows[0].order_ref} marked paid via phone match fallback.`);
            sendOrderConfirmation(fallbackResult.rows[0]).catch(e => console.warn('Paid fallback email error:', e.message));
          }
        }
      }
    } else {
      console.warn(`❌ M-Pesa payment failed — ${resultDesc} (ResultCode: ${resultCode})`);
      // Update order to failed if customer cancelled or failed
      if (checkoutRequestId) {
        await pgPool.query(
          `UPDATE orders 
           SET payment_status = 'failed'
           WHERE mpesa_checkout_request_id = $1 AND payment_status = 'pending'`,
          [checkoutRequestId]
        );
      }
    }

    // Always respond 200 to Safaricom so they stop retrying
    res.json({ ResultCode: 0, ResultDesc: 'Accepted' });
  } catch (err) {
    console.error('M-Pesa callback error:', err.message);
    res.status(500).json({ message: err.message });
  }
});

// ─── POST /api/mpesa/query ────────────────────────────────────────────────────
// Poll the STK push status (used by frontend to check if user has paid).
router.post('/query', async (req, res) => {
  try {
    const { checkoutRequestId } = req.body;
    if (!checkoutRequestId) return res.status(400).json({ message: 'checkoutRequestId is required' });

    const token = await getAccessToken();
    const { timestamp, password } = getTimestampAndPassword();

    const queryRes = await fetch(`${MPESA_BASE_URL}/mpesa/stkpushquery/v1/query`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        BusinessShortCode: MPESA_SHORTCODE,
        Password: password,
        Timestamp: timestamp,
        CheckoutRequestID: checkoutRequestId,
      }),
    });

    const data = await queryRes.json();
    const isPaid = data.ResultCode === '0';

    // If query confirms payment, update database proactively
    if (isPaid && checkoutRequestId) {
      await pgPool.query(
        `UPDATE orders 
         SET payment_status = 'paid',
             status = CASE WHEN status = 'pending' THEN 'confirmed' ELSE status END
         WHERE mpesa_checkout_request_id = $1 AND payment_status = 'pending'`,
        [checkoutRequestId]
      );
    } else if (data.ResultCode && data.ResultCode !== '0' && data.ResultCode !== '1037') {
      // 1037 is timeout / in-progress; other codes mean failed or cancelled
      await pgPool.query(
        `UPDATE orders 
         SET payment_status = 'failed'
         WHERE mpesa_checkout_request_id = $1 AND payment_status = 'pending'`,
        [checkoutRequestId]
      );
    }

    res.json({
      resultCode: data.ResultCode,
      resultDesc: data.ResultDesc,
      paid: isPaid,
    });
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

// ─── GET /api/mpesa/order-status/:orderRef ────────────────────────────────────
// Lightweight status polling endpoint for frontend order verification
router.get('/order-status/:orderRef', async (req, res) => {
  try {
    const { orderRef } = req.params;
    const { rows } = await pgPool.query(
      `SELECT id, order_ref, status, payment_status, mpesa_transaction_id, total_amount, created_at 
       FROM orders WHERE order_ref = $1`,
      [orderRef]
    );

    if (rows.length === 0) {
      return res.status(404).json({ message: 'Order not found' });
    }

    res.json(rows[0]);
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

module.exports = router;
