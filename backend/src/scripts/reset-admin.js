require('dotenv').config();
const bcrypt = require('bcryptjs');
const { pgPool } = require('../config/db');

const resetAdmin = async () => {
  const newPassword = process.argv[2];
  const adminEmail = process.argv[3] || 'admin@naojaventures.com';

  if (!newPassword) {
    console.error('❌ Error: Please provide the new password.');
    console.log('Usage: node src/scripts/reset-admin.js <newPassword> [adminEmail]');
    process.exit(1);
  }

  if (newPassword.length < 6) {
    console.error('❌ Error: Password must be at least 6 characters.');
    process.exit(1);
  }

  try {
    const salt = await bcrypt.genSalt(10);
    const hashedPassword = await bcrypt.hash(newPassword, salt);

    // Check if admin exists
    const check = await pgPool.query('SELECT id, email, name FROM admins WHERE email = $1', [adminEmail]);

    if (check.rows.length === 0) {
      await pgPool.query(
        'INSERT INTO admins (name, email, password) VALUES ($1, $2, $3)',
        ['Super Admin', adminEmail, hashedPassword]
      );
      console.log(`✅ Created admin "${adminEmail}" with the new password.`);
    } else {
      await pgPool.query(
        'UPDATE admins SET password = $1 WHERE email = $2',
        [hashedPassword, adminEmail]
      );
      console.log(`✅ Password updated successfully for admin: ${adminEmail}`);
    }

    process.exit(0);
  } catch (err) {
    console.error('❌ Failed to update admin password:', err.message);
    process.exit(1);
  }
};

resetAdmin();
