const nodemailer = require('nodemailer');

// Create transporter only if email credentials are configured
let transporter = null;

const getTransporter = () => {
  if (transporter) return transporter;

  const emailUser = process.env.EMAIL_USER;
  const emailPass = process.env.EMAIL_PASS;

  if (!emailUser || !emailPass || emailUser === 'your_gmail@gmail.com') {
    return null;
  }

  try {
    transporter = nodemailer.createTransport({
      service: process.env.EMAIL_SERVICE || 'gmail',
      auth: {
        user: emailUser,
        pass: emailPass
      }
    });
    return transporter;
  } catch (err) {
    console.warn('⚠️ Could not initialize email transporter:', err.message);
    return null;
  }
};

/**
 * Format currency in KES
 */
const formatKes = (amount) => {
  return new Intl.NumberFormat('en-KE', { style: 'currency', currency: 'KES', maximumFractionDigits: 0 }).format(amount || 0);
};

/**
 * Parse items JSON safely
 */
const parseItems = (rawItems) => {
  if (!rawItems) return [];
  if (Array.isArray(rawItems)) return rawItems;
  try {
    return JSON.parse(rawItems);
  } catch (e) {
    return [];
  }
};

/**
 * Send order receipt email to customer
 */
const sendOrderConfirmation = async (order) => {
  if (!order || !order.customer_email) return false;

  const mailer = getTransporter();
  if (!mailer) {
    console.log(`ℹ️ [Email Service] Skipping customer email for ${order.order_ref} (SMTP not configured)`);
    return false;
  }

  const items = parseItems(order.items);
  const itemsHtml = items.map(item => `
    <tr style="border-bottom: 1px solid #e5e7eb;">
      <td style="padding: 10px 0; font-size: 14px; color: #111827;">${item.product_name || item.name || 'Product'}</td>
      <td style="padding: 10px 0; font-size: 14px; text-align: center; color: #4b5563;">${item.quantity}</td>
      <td style="padding: 10px 0; font-size: 14px; text-align: right; color: #111827; font-weight: 600;">${formatKes((item.unit_price || item.price || 0) * item.quantity)}</td>
    </tr>
  `).join('');

  const html = `
    <div style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; max-width: 600px; margin: 0 auto; background: #ffffff; border: 1px solid #e5e7eb; border-radius: 12px; overflow: hidden;">
      <div style="background-color: #15803d; padding: 24px; text-align: center; color: #ffffff;">
        <h1 style="margin: 0; font-size: 24px; font-weight: 800; letter-spacing: -0.5px;">Naoja Ventures</h1>
        <p style="margin: 6px 0 0; font-size: 13px; opacity: 0.9;">Retail & Wholesale Shop · Lurambi, Kakamega</p>
      </div>

      <div style="padding: 24px;">
        <h2 style="font-size: 18px; color: #111827; margin-top: 0;">Thank you for your order, ${order.customer_name}!</h2>
        <p style="color: #4b5563; font-size: 14px; line-height: 1.5;">
          Your order has been received and is currently being processed. Here is your summary:
        </p>

        <div style="background: #f9fafb; border-radius: 8px; padding: 16px; margin: 20px 0; font-size: 13px;">
          <p style="margin: 4px 0;"><strong>Order Reference:</strong> <span style="color: #15803d; font-weight: 700;">${order.order_ref}</span></p>
          <p style="margin: 4px 0;"><strong>Payment Status:</strong> ${order.payment_status?.toUpperCase() || 'PENDING'}</p>
          <p style="margin: 4px 0;"><strong>Payment Method:</strong> ${order.payment_method === 'mpesa' ? 'M-Pesa (Buy Goods Till 4149)' : order.payment_method}</p>
          ${order.mpesa_transaction_id ? `<p style="margin: 4px 0;"><strong>M-Pesa Receipt:</strong> ${order.mpesa_transaction_id}</p>` : ''}
          ${order.shipping_address ? `<p style="margin: 4px 0;"><strong>Delivery Address:</strong> ${order.shipping_address}</p>` : ''}
          <p style="margin: 4px 0;"><strong>Phone:</strong> ${order.customer_phone}</p>
        </div>

        <h3 style="font-size: 15px; color: #111827; margin-bottom: 10px; border-bottom: 2px solid #f3f4f6; padding-bottom: 8px;">Order Items</h3>
        <table style="width: 100%; border-collapse: collapse; margin-bottom: 20px;">
          <thead>
            <tr style="border-bottom: 1px solid #d1d5db; text-align: left; font-size: 12px; color: #6b7280; text-transform: uppercase;">
              <th style="padding-bottom: 8px;">Item</th>
              <th style="padding-bottom: 8px; text-align: center;">Qty</th>
              <th style="padding-bottom: 8px; text-align: right;">Total</th>
            </tr>
          </thead>
          <tbody>
            ${itemsHtml}
          </tbody>
        </table>

        <div style="border-top: 2px solid #111827; padding-top: 14px; display: flex; justify-content: space-between; font-size: 16px;">
          <p style="margin: 0; font-weight: 800; color: #111827;">Total Amount:</p>
          <p style="margin: 0; font-weight: 800; color: #15803d; font-size: 18px; text-align: right;">${formatKes(order.total_amount)}</p>
        </div>

        <div style="margin-top: 30px; padding: 16px; background: #ecfdf5; border: 1px solid #a7f3d0; border-radius: 8px; font-size: 13px; color: #065f46;">
          <strong>Need assistance with this order?</strong><br/>
          Contact our support team directly via WhatsApp or Phone at <strong>+254 712 345 678</strong>.
        </div>
      </div>

      <div style="background: #f3f4f6; padding: 16px; text-align: center; font-size: 12px; color: #6b7280;">
        &copy; ${new Date().getFullYear()} Naoja Ventures. All rights reserved.
      </div>
    </div>
  `;

  try {
    await mailer.sendMail({
      from: process.env.EMAIL_FROM || `Naoja Ventures <${process.env.EMAIL_USER}>`,
      to: order.customer_email,
      subject: `Order Confirmation — ${order.order_ref} | Naoja Ventures`,
      html
    });
    console.log(`✉️ Order confirmation email sent to ${order.customer_email} for ${order.order_ref}`);
    return true;
  } catch (err) {
    console.error(`⚠️ Failed to send order confirmation email:`, err.message);
    return false;
  }
};

/**
 * Send alert to store owner / admin
 */
const sendAdminOrderAlert = async (order) => {
  const adminEmail = process.env.ADMIN_EMAIL || process.env.EMAIL_USER;
  if (!adminEmail || adminEmail === 'your_gmail@gmail.com') return false;

  const mailer = getTransporter();
  if (!mailer) return false;

  const items = parseItems(order.items);
  const itemsText = items.map(i => `- ${i.product_name || i.name || 'Product'} (x${i.quantity}) - ${formatKes((i.unit_price || i.price || 0) * i.quantity)}`).join('\n');

  try {
    await mailer.sendMail({
      from: process.env.EMAIL_FROM || `Naoja Shop <${process.env.EMAIL_USER}>`,
      to: adminEmail,
      subject: `🚨 New Order: ${order.order_ref} (${formatKes(order.total_amount)})`,
      text: `A new order has been placed on Naoja Ventures!

Order Ref: ${order.order_ref}
Customer: ${order.customer_name}
Phone: ${order.customer_phone}
Email: ${order.customer_email || 'N/A'}
Total: ${formatKes(order.total_amount)}
Payment: ${order.payment_method} (${order.payment_status})
Address: ${order.shipping_address || 'Store pickup / Kakamega'}

Items:
${itemsText}

Log into the admin panel to view full details.
`
    });
    console.log(`✉️ Admin order notification sent to ${adminEmail}`);
    return true;
  } catch (err) {
    console.error(`⚠️ Failed to send admin order alert:`, err.message);
    return false;
  }
};

module.exports = {
  sendOrderConfirmation,
  sendAdminOrderAlert
};
