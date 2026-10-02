const mongoose = require('mongoose');
const { Pool } = require('pg');

// Initialize PostgreSQL connection pool
const pgPool = new Pool({
  host: process.env.PG_HOST,
  port: process.env.PG_PORT,
  user: process.env.PG_USER,
  password: process.env.PG_PASSWORD,
  database: process.env.PG_DATABASE,
});

pgPool.on('error', (err, client) => {
  console.error('Unexpected error on idle PostgreSQL client', err);
});

// Function to connect to both databases
const connectDatabases = async () => {
  try {
    // 1. Connect MongoDB
    if (!process.env.MONGODB_URI) {
      console.warn('⚠️ MONGODB_URI not set in .env');
    } else {
      mongoose.set('bufferCommands', false);
      await mongoose.connect(process.env.MONGODB_URI, {
        serverSelectionTimeoutMS: 2500,
        connectTimeoutMS: 2500,
      });
      console.log('✅ MongoDB Connected Successfully');
    }

    // 2. Connect PostgreSQL
    if (!process.env.PG_HOST || !process.env.PG_DATABASE) {
      console.warn('⚠️ PostgreSQL configuration missing in .env');
    } else {
      const pgClient = await pgPool.connect();
      console.log('✅ PostgreSQL Connected Successfully');
      pgClient.release(); // release the client back to the pool
    }
  } catch (error) {
    console.error('❌ Database connection failed:', error.message);
    // Don't exit process strictly yet to allow fallback mock testing if DB isn't running on user's machine
  }
};

module.exports = {
  connectDatabases,
  pgPool
};
