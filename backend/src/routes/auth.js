const express = require('express');
const router = express.Router();
const bcrypt = require('bcryptjs');
const jwt = require('jsonwebtoken');
const { pgPool } = require('../config/db');

router.post('/login', async (req, res) => {
  try {
    const { email, password } = req.body;

    if (!email || !password) {
      return res.status(400).json({ message: 'Please provide email and password' });
    }

    const { rows } = await pgPool.query('SELECT * FROM admins WHERE email = $1', [email]);
    if (rows.length === 0) {
      return res.status(400).json({ message: 'Invalid credentials' });
    }

    const admin = rows[0];
    const isMatch = await bcrypt.compare(password, admin.password);

    if (!isMatch) {
      return res.status(400).json({ message: 'Invalid credentials' });
    }

    const jwtSecret = process.env.JWT_SECRET || (process.env.NODE_ENV === 'production' ? null : 'fallback_dev_secret_key_123');
    if (!jwtSecret) {
      console.error('CRITICAL: JWT_SECRET environment variable must be set in production!');
      return res.status(500).json({ message: 'Server configuration error' });
    }

    const token = jwt.sign(
      { id: admin.id, email: admin.email, name: admin.name },
      jwtSecret,
      { expiresIn: '1d' }
    );

    res.json({
      token,
      admin: {
        id: admin.id,
        name: admin.name,
        email: admin.email
      }
    });
  } catch (error) {
    res.status(500).json({ message: error.message });
  }
});

module.exports = router;
