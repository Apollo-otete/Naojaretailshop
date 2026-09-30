require('dotenv').config();
const express = require('express');
const cors = require('cors');
const path = require('path');
const fs = require('fs');
const rateLimit = require('express-rate-limit');
const { connectDatabases } = require('./src/config/db');
const { initPgTables } = require('./src/config/initPg');

const app = express();
const PORT = process.env.PORT || 5000;

// Ensure uploads dir exists
const uploadsDir = path.join(__dirname, 'uploads');
if (!fs.existsSync(uploadsDir)) fs.mkdirSync(uploadsDir, { recursive: true });

// CORS Configuration
const allowedOrigins = process.env.FRONTEND_URL
  ? [process.env.FRONTEND_URL, 'http://localhost:5173', 'http://localhost:3000']
  : '*';

app.use(cors({
  origin: (origin, callback) => {
    if (!origin || allowedOrigins === '*' || allowedOrigins.includes(origin)) {
      callback(null, true);
    } else {
      callback(new Error('Blocked by CORS policy'));
    }
  },
  credentials: true
}));

// Body Parsers with payload size limits
app.use(express.json({ limit: '5mb' }));
app.use(express.urlencoded({ extended: true, limit: '5mb' }));

// Rate Limiters
const authLimiter = rateLimit({
  windowMs: 15 * 60 * 1000, // 15 minutes
  max: 15, // limit each IP to 15 login requests per windowMs
  standardHeaders: true,
  legacyHeaders: false,
  message: { message: 'Too many login attempts. Please try again after 15 minutes.' }
});

const mpesaLimiter = rateLimit({
  windowMs: 60 * 1000, // 1 minute
  max: 6, // limit each IP to 6 STK pushes per minute to avoid spam / toll fraud
  standardHeaders: true,
  legacyHeaders: false,
  message: { message: 'Too many payment requests. Please wait a minute before trying again.' }
});

// Serve uploaded product images as static files
app.use('/uploads', express.static(uploadsDir));

// Health check endpoint for uptime monitors & load balancers
app.get('/api/health', (req, res) => {
  res.json({
    status: 'healthy',
    timestamp: new Date().toISOString(),
    uptime: process.uptime()
  });
});

// Mount routes with rate limiting
app.use('/api/auth/login', authLimiter);
app.use('/api/auth', require('./src/routes/auth'));
app.use('/api/mpesa/stkpush', mpesaLimiter);
app.use('/api/mpesa', require('./src/routes/mpesa'));
app.use('/api', require('./src/routes'));

// Global error handler
app.use((err, req, res, next) => {
  console.error('Unhandled Server Error:', err.message);
  res.status(err.status || 500).json({
    message: err.message || 'Internal Server Error'
  });
});

// Start server
const startServer = async () => {
  await connectDatabases();
  await initPgTables();
  
  const server = app.listen(PORT, () => {
    console.log(`✅ Server running on http://localhost:${PORT}`);
  });

  // Graceful shutdown
  const shutdown = () => {
    console.log('Stopping server gracefully...');
    server.close(() => {
      console.log('HTTP server closed.');
      process.exit(0);
    });
  };

  process.on('SIGTERM', shutdown);
  process.on('SIGINT', shutdown);
};

startServer();