const jwt = require('jsonwebtoken');

const authMiddleware = (req, res, next) => {
  const token = req.header('Authorization');

  if (!token) {
    return res.status(401).json({ message: 'No authentication token, access denied' });
  }

  try {
    const cleanToken = token.startsWith('Bearer ') ? token.slice(7, token.length) : token;
    const verified = jwt.verify(cleanToken, process.env.JWT_SECRET || 'fallback_secret_key_123');
    req.admin = verified;
    next();
  } catch (err) {
    res.status(401).json({ message: 'Token verification failed, authorization denied' });
  }
};

module.exports = authMiddleware;
