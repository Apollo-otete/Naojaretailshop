const jwt = require('jsonwebtoken');

const authMiddleware = (req, res, next) => {
  const token = req.header('Authorization') || req.query.token;

  if (!token) {
    return res.status(401).json({ message: 'No authentication token, access denied' });
  }

  try {
    const cleanToken = token.startsWith('Bearer ') ? token.slice(7, token.length) : token;
    const jwtSecret = process.env.JWT_SECRET || (process.env.NODE_ENV === 'production' ? null : 'fallback_dev_secret_key_123');
    if (!jwtSecret) {
      return res.status(500).json({ message: 'Authentication misconfigured' });
    }
    const verified = jwt.verify(cleanToken, jwtSecret);
    req.admin = verified;
    next();
  } catch (err) {
    res.status(401).json({ message: 'Token verification failed, authorization denied' });
  }
};

module.exports = authMiddleware;
