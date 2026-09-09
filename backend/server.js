require('dotenv').config();
const express = require('express');
const cors = require('cors');
const path = require('path');
const fs = require('fs');
const { connectDatabases } = require('./src/config/db');
const { initPgTables } = require('./src/config/initPg');

const app = express();
const PORT = process.env.PORT || 5000;

// Ensure uploads dir exists
const uploadsDir = path.join(__dirname, 'uploads');
if (!fs.existsSync(uploadsDir)) fs.mkdirSync(uploadsDir, { recursive: true });

// Middleware
app.use(cors());
app.use(express.json());
app.use(express.urlencoded({ extended: true }));

// Serve uploaded product images as static files
app.use('/uploads', express.static(uploadsDir));

// Import routes
app.use('/api/auth', require('./src/routes/auth'));
app.use('/api/mpesa', require('./src/routes/mpesa'));
app.use('/api', require('./src/routes'));

// Start server
const startServer = async () => {
  await connectDatabases();
  await initPgTables();
  
  app.listen(PORT, () => {
    console.log(`✅ Server running on http://localhost:${PORT}`);
  });
};

startServer();