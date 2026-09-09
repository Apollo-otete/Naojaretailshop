const mongoose = require('mongoose');

const productSchema = new mongoose.Schema({
  id: { type: Number, required: true, unique: true },
  name: { type: String, required: true },
  slug: { type: String, required: true, unique: true },
  description: { type: String },
  price: { type: Number, required: true },
  stock_quantity: { type: Number, required: true, default: 0 },
  category_id: { type: Number, required: true },
  subcategory: { type: String },
  images: [{ type: String }],
  status: { type: String, enum: ['in_stock', 'low_stock', 'out_of_stock'], default: 'in_stock' },
  is_featured: { type: Boolean, default: false },
  views: { type: Number, default: 0 },
  rating_avg: { type: Number, default: 0 },
  rating_count: { type: Number, default: 0 }
}, {
  timestamps: true
});

module.exports = mongoose.model('Product', productSchema);
