const mongoose = require('mongoose');

const reviewSchema = new mongoose.Schema({
  id: { type: Number, required: true, unique: true },
  product_id: { type: Number, required: true },
  name: { type: String, required: true },
  rating: { type: Number, required: true, min: 1, max: 5 },
  comment: { type: String },
  is_approved: { type: Boolean, default: false },
  created_at: { type: Date, default: Date.now }
});

module.exports = mongoose.model('Review', reviewSchema);
