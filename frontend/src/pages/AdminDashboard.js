import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../lib/api';
import {
  LayoutDashboard,
  ShoppingBag,
  FolderTree,
  Receipt,
  Star,
  MessageSquare,
  Users,
  Plus,
  Edit,
  Trash2,
  Check,
  X,
  Search,
  AlertTriangle,
  TrendingUp,
  ExternalLink,
  Clock,
  ArrowLeft,
  ChevronRight,
  Eye,
  CheckCircle,
  XCircle,
  Menu
} from 'lucide-react';

export default function AdminDashboard() {
  const [activeTab, setActiveTab] = useState('overview');
  const [stats, setStats] = useState({
    totalSales: 0,
    totalOrders: 0,
    totalProducts: 0,
    totalSubscribers: 0,
    lowStockCount: 0,
    pendingMessagesCount: 0,
    recentOrders: [],
    monthlySales: []
  });
  const [products, setProducts] = useState([]);
  const [categories, setCategories] = useState([]);
  const [orders, setOrders] = useState([]);
  const [reviews, setReviews] = useState([]);
  const [subscribers, setSubscribers] = useState([]);
  const [messages, setMessages] = useState([]);
  
  const [loading, setLoading] = useState(true);
  const [sidebarOpen, setSidebarOpen] = useState(false);

  // Search & Filter States
  const [prodSearch, setProdSearch] = useState('');
  const [prodCategoryFilter, setProdCategoryFilter] = useState('');
  const [orderSearch, setOrderSearch] = useState('');
  const [orderStatusFilter, setOrderStatusFilter] = useState('');
  const [orderPaymentFilter, setOrderPaymentFilter] = useState('');

  // Modals States
  const [selectedOrder, setSelectedOrder] = useState(null);
  const [isProductModalOpen, setIsProductModalOpen] = useState(false);
  const [editingProduct, setEditingProduct] = useState(null);
  const [isCategoryModalOpen, setIsCategoryModalOpen] = useState(false);
  const [editingCategory, setEditingCategory] = useState(null);

  // Form States - Product
  const [productForm, setProductForm] = useState({
    name: '',
    slug: '',
    description: '',
    price: '',
    stock_quantity: '',
    category_id: '',
    subcategory: '',
    image_url: ''
  });

  // Form States - Category
  const [categoryForm, setCategoryForm] = useState({
    name: '',
    slug: '',
    icon: '📁'
  });

  // Fetch Data
  const fetchData = async () => {
    setLoading(true);
    try {
      const [
        analyticsData,
        productsData,
        categoriesData,
        ordersData,
        reviewsData,
        subscribersData,
        messagesData
      ] = await Promise.all([
        api.admin.getAnalytics(),
        api.admin.getProducts(),
        api.getCategories(),
        api.admin.getOrders(),
        api.admin.getReviews(),
        api.admin.getSubscribers(),
        api.admin.getMessages()
      ]);

      if (analyticsData) setStats(analyticsData);
      if (productsData) setProducts(productsData);
      if (categoriesData) setCategories(categoriesData);
      if (ordersData) setOrders(ordersData);
      if (reviewsData) setReviews(reviewsData);
      if (subscribersData) setSubscribers(subscribersData);
      if (messagesData) setMessages(messagesData);
    } catch (err) {
      console.error('Error fetching admin data:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  // Sync Analytics with local updates
  const refreshAnalytics = (updatedProducts = products, updatedOrders = orders, updatedSubscribers = subscribers, updatedMessages = messages) => {
    const totalSales = updatedOrders
      .filter(o => o.payment_status === 'paid' || o.status === 'delivered')
      .reduce((acc, o) => acc + o.total_amount, 0);

    const pendingMessagesCount = updatedMessages.filter(m => !m.is_read).length;
    const lowStockCount = updatedProducts.filter(p => p.stock_quantity <= 5).length;

    setStats(prev => ({
      ...prev,
      totalSales,
      totalOrders: updatedOrders.length,
      totalProducts: updatedProducts.length,
      totalSubscribers: updatedSubscribers.length,
      lowStockCount,
      pendingMessagesCount,
      recentOrders: [...updatedOrders].sort((a,b) => new Date(b.created_at) - new Date(a.created_at)).slice(0, 5)
    }));
  };

  // Product Actions
  const handleOpenProductModal = (prod = null) => {
    if (prod) {
      setEditingProduct(prod);
      setProductForm({
        name: prod.name,
        slug: prod.slug,
        description: prod.description || '',
        price: prod.price,
        stock_quantity: prod.stock_quantity,
        category_id: prod.category_id || '',
        subcategory: prod.subcategory || '',
        image_url: prod.images && prod.images[0] ? prod.images[0] : ''
      });
    } else {
      setEditingProduct(null);
      setProductForm({
        name: '',
        slug: '',
        description: '',
        price: '',
        stock_quantity: '',
        category_id: categories.length > 0 ? categories[0].id : '',
        subcategory: '',
        image_url: ''
      });
    }
    setIsProductModalOpen(true);
  };

  const handleProductSubmit = async (e) => {
    e.preventDefault();
    const payload = {
      name: productForm.name,
      slug: productForm.slug || productForm.name.toLowerCase().replace(/[^a-z0-9]+/g, '-'),
      description: productForm.description,
      price: parseFloat(productForm.price),
      stock_quantity: parseInt(productForm.stock_quantity),
      category_id: parseInt(productForm.category_id),
      subcategory: productForm.subcategory,
      images: productForm.image_url ? [productForm.image_url] : ['https://images.unsplash.com/photo-1505740420928-5e560c06d30e?w=600']
    };

    try {
      let updatedList = [];
      if (editingProduct) {
        const res = await api.admin.editProduct(editingProduct.id, payload);
        updatedList = products.map(p => p.id === editingProduct.id ? res : p);
        setProducts(updatedList);
      } else {
        const res = await api.admin.addProduct(payload);
        updatedList = [...products, res];
        setProducts(updatedList);
      }
      refreshAnalytics(updatedList, orders, subscribers, messages);
      setIsProductModalOpen(false);
    } catch (err) {
      alert('Failed to save product: ' + err.message);
    }
  };

  const handleDeleteProduct = async (id) => {
    if (!window.confirm('Are you sure you want to delete this product?')) return;
    try {
      await api.admin.deleteProduct(id);
      const updatedList = products.filter(p => p.id !== id);
      setProducts(updatedList);
      refreshAnalytics(updatedList, orders, subscribers, messages);
    } catch (err) {
      alert('Failed to delete product: ' + err.message);
    }
  };

  // Category Actions
  const handleOpenCategoryModal = (cat = null) => {
    if (cat) {
      setEditingCategory(cat);
      setCategoryForm({
        name: cat.name,
        slug: cat.slug,
        icon: cat.icon || '📁'
      });
    } else {
      setEditingCategory(null);
      setCategoryForm({
        name: '',
        slug: '',
        icon: '📁'
      });
    }
    setIsCategoryModalOpen(true);
  };

  const handleCategorySubmit = async (e) => {
    e.preventDefault();
    const payload = {
      name: categoryForm.name,
      slug: categoryForm.slug || categoryForm.name.toLowerCase().replace(/[^a-z0-9]+/g, '-'),
      icon: categoryForm.icon
    };

    try {
      if (editingCategory) {
        const res = await api.admin.editCategory(editingCategory.id, payload);
        setCategories(categories.map(c => c.id === editingCategory.id ? res : c));
      } else {
        const res = await api.admin.addCategory(payload);
        setCategories([...categories, res]);
      }
      setIsCategoryModalOpen(false);
    } catch (err) {
      alert('Failed to save category: ' + err.message);
    }
  };

  const handleDeleteCategory = async (id) => {
    if (!window.confirm('Deleting a category will not delete its products but may leave them uncategorized. Proceed?')) return;
    try {
      await api.admin.deleteCategory(id);
      setCategories(categories.filter(c => c.id !== id));
    } catch (err) {
      alert('Failed to delete category: ' + err.message);
    }
  };

  // Order Actions
  const handleUpdateOrderStatus = async (id, status) => {
    try {
      const res = await api.admin.updateOrderStatus(id, status);
      const updatedList = orders.map(o => o.id === id ? res : o);
      setOrders(updatedList);
      if (selectedOrder && selectedOrder.id === id) {
        setSelectedOrder(res);
      }
      refreshAnalytics(products, updatedList, subscribers, messages);
    } catch (err) {
      alert('Failed to update status: ' + err.message);
    }
  };

  const handleUpdateOrderPaymentStatus = async (id, paymentStatus) => {
    try {
      const res = await api.admin.updateOrderPaymentStatus(id, paymentStatus);
      const updatedList = orders.map(o => o.id === id ? res : o);
      setOrders(updatedList);
      if (selectedOrder && selectedOrder.id === id) {
        setSelectedOrder(res);
      }
      refreshAnalytics(products, updatedList, subscribers, messages);
    } catch (err) {
      alert('Failed to update payment status: ' + err.message);
    }
  };

  // Review Actions
  const handleApproveReview = async (id) => {
    try {
      const res = await api.admin.approveReview(id);
      setReviews(reviews.map(r => r.id === id ? res : r));
    } catch (err) {
      alert('Failed to approve review: ' + err.message);
    }
  };

  const handleRejectReview = async (id) => {
    if (!window.confirm('Delete/Reject this review?')) return;
    try {
      await api.admin.rejectReview(id);
      setReviews(reviews.filter(r => r.id !== id));
    } catch (err) {
      alert('Failed to delete review: ' + err.message);
    }
  };

  // Message Actions
  const handleToggleMessageRead = async (msg) => {
    const updatedMessages = messages.map(m => m.id === msg.id ? { ...m, is_read: !m.is_read } : m);
    setMessages(updatedMessages);
    // Simulating endpoint persistence in fallback
    const db = JSON.parse(localStorage.getItem('naoja_db'));
    if (db) {
      db.messages = db.messages.map(m => m.id === msg.id ? { ...m, is_read: !msg.is_read } : m);
      localStorage.setItem('naoja_db', JSON.stringify(db));
    }
    refreshAnalytics(products, orders, subscribers, updatedMessages);
  };

  // Sidebar Menu Items
  const menuItems = [
    { id: 'overview', label: 'Overview', icon: LayoutDashboard },
    { id: 'products', label: 'Products', icon: ShoppingBag, badge: stats.lowStockCount ? stats.lowStockCount : null },
    { id: 'categories', label: 'Categories', icon: FolderTree },
    { id: 'orders', label: 'Orders', icon: Receipt },
    { id: 'reviews', label: 'Reviews', icon: Star, badge: reviews.filter(r => !r.is_approved).length ? reviews.filter(r => !r.is_approved).length : null },
    { id: 'messages', label: 'Inquiries', icon: MessageSquare, badge: stats.pendingMessagesCount ? stats.pendingMessagesCount : null },
    { id: 'subscribers', label: 'Subscribers', icon: Users }
  ];

  // Filters logic
  const filteredProducts = products.filter(p => {
    const matchesSearch = p.name.toLowerCase().includes(prodSearch.toLowerCase()) || p.slug.toLowerCase().includes(prodSearch.toLowerCase());
    const matchesCategory = prodCategoryFilter ? p.category_id === parseInt(prodCategoryFilter) : true;
    return matchesSearch && matchesCategory;
  });

  const filteredOrders = orders.filter(o => {
    const matchesSearch = o.order_ref.toLowerCase().includes(orderSearch.toLowerCase()) || o.customer_name.toLowerCase().includes(orderSearch.toLowerCase());
    const matchesStatus = orderStatusFilter ? o.status === orderStatusFilter : true;
    const matchesPayment = orderPaymentFilter ? o.payment_status === orderPaymentFilter : true;
    return matchesSearch && matchesStatus && matchesPayment;
  });

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col font-sans">
      
      {/* Top Banner Navigation */}
      <header className="bg-white border-b border-gray-200 h-16 shrink-0 flex items-center justify-between px-4 sticky top-0 z-20">
        <div className="flex items-center gap-3">
          <button 
            onClick={() => setSidebarOpen(!sidebarOpen)}
            className="p-1.5 hover:bg-gray-100 rounded-lg lg:hidden"
          >
            <Menu className="w-6 h-6 text-gray-700" />
          </button>
          <span className="font-serif text-xl font-bold text-brand-600 flex items-center gap-2">
            Naoja Ventures <span className="bg-brand-100 text-brand-800 text-[10px] uppercase font-bold px-2 py-0.5 rounded-full">Admin</span>
          </span>
        </div>

        <div className="flex items-center gap-4">
          <Link to="/" className="text-xs font-semibold text-gray-600 hover:text-brand-600 flex items-center gap-1.5 transition-colors">
            <ArrowLeft className="w-3.5 h-3.5" />
            Back to Storefront
          </Link>
        </div>
      </header>

      <div className="flex flex-1 relative overflow-hidden">
        
        {/* Sidebar Navigation */}
        <aside className={`bg-white border-r border-gray-200 w-64 shrink-0 flex flex-col justify-between py-6 absolute inset-y-0 left-0 z-10 transform lg:static lg:translate-x-0 transition-transform duration-300 ${
          sidebarOpen ? 'translate-x-0' : '-translate-x-full'
        }`}>
          <div className="space-y-1.5 px-3">
            {menuItems.map((item) => {
              const Icon = item.icon;
              return (
                <button
                  key={item.id}
                  onClick={() => {
                    setActiveTab(item.id);
                    setSidebarOpen(false);
                  }}
                  className={`w-full h-11 px-4 rounded-xl flex items-center justify-between font-semibold text-sm transition-all ${
                    activeTab === item.id 
                      ? 'bg-brand-50 text-brand-700 shadow-sm' 
                      : 'text-gray-600 hover:bg-gray-50'
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <Icon className={`w-5 h-5 ${activeTab === item.id ? 'text-brand-600' : 'text-gray-400'}`} />
                    {item.label}
                  </div>
                  {item.badge && (
                    <span className="bg-brand-600 text-white text-[10px] font-extrabold px-2 py-0.5 rounded-full">
                      {item.badge}
                    </span>
                  )}
                </button>
              );
            })}
          </div>

          <div className="px-4 text-[10px] text-gray-400 font-semibold tracking-wider uppercase">
            Kakamega Shop Operations
          </div>
        </aside>

        {/* Overlay backdrop for mobile sidebar */}
        {sidebarOpen && (
          <div 
            onClick={() => setSidebarOpen(false)} 
            className="bg-black/20 absolute inset-0 z-0 lg:hidden"
          ></div>
        )}

        {/* Main Content Area */}
        <main className="flex-1 overflow-y-auto p-4 lg:p-8">
          {loading ? (
            <div className="flex flex-col items-center justify-center py-24 text-gray-400 font-medium space-y-2">
              <span className="text-sm">Loading database parameters...</span>
            </div>
          ) : (
            <>
              {/* Tab 1: OVERVIEW */}
              {activeTab === 'overview' && (
                <div className="space-y-6">
                  <div>
                    <h1 className="font-serif text-2xl lg:text-3xl font-bold text-gray-900">Dashboard Overview</h1>
                    <p className="text-sm text-gray-500 mt-1">Real-time telemetry and retail sales statistics.</p>
                  </div>

                  {/* Stock Warnings */}
                  {stats.lowStockCount > 0 && (
                    <div className="bg-amber-50 border border-amber-200 rounded-2xl p-4 flex items-start gap-3.5 shadow-sm">
                      <AlertTriangle className="w-5 h-5 text-amber-600 shrink-0 mt-0.5" />
                      <div>
                        <h4 className="font-bold text-amber-800 text-sm">Low Stock Alert</h4>
                        <p className="text-xs text-amber-600 mt-0.5">
                          There are currently <strong>{stats.lowStockCount}</strong> products running low on inventory (5 units or less). Review catalog stock levels.
                        </p>
                      </div>
                    </div>
                  )}

                  {/* Stat Cards Grid */}
                  <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5">
                    <div className="bg-white border border-gray-200 rounded-2xl p-6 shadow-sm flex flex-col justify-between">
                      <div className="flex justify-between items-start">
                        <div>
                          <span className="text-xs font-bold text-gray-400 uppercase tracking-wider">Total Revenue</span>
                          <h2 className="text-2xl font-extrabold text-gray-900 mt-1.5">
                            KSh {stats.totalSales.toLocaleString()}
                          </h2>
                        </div>
                        <div className="p-2.5 bg-green-50 text-green-600 rounded-xl">
                          <TrendingUp className="w-5 h-5" />
                        </div>
                      </div>
                      <div className="text-[10px] text-green-600 font-semibold mt-4 flex items-center gap-1">
                        <span>Paid & Delivered Orders</span>
                      </div>
                    </div>

                    <div className="bg-white border border-gray-200 rounded-2xl p-6 shadow-sm flex flex-col justify-between">
                      <div className="flex justify-between items-start">
                        <div>
                          <span className="text-xs font-bold text-gray-400 uppercase tracking-wider">Total Orders</span>
                          <h2 className="text-2xl font-extrabold text-gray-900 mt-1.5">
                            {stats.totalOrders}
                          </h2>
                        </div>
                        <div className="p-2.5 bg-blue-50 text-blue-600 rounded-xl">
                          <Receipt className="w-5 h-5" />
                        </div>
                      </div>
                      <div className="text-[10px] text-gray-500 font-semibold mt-4">
                        All transaction attempts
                      </div>
                    </div>

                    <div className="bg-white border border-gray-200 rounded-2xl p-6 shadow-sm flex flex-col justify-between">
                      <div className="flex justify-between items-start">
                        <div>
                          <span className="text-xs font-bold text-gray-400 uppercase tracking-wider">Active Catalog</span>
                          <h2 className="text-2xl font-extrabold text-gray-900 mt-1.5">
                            {stats.totalProducts}
                          </h2>
                        </div>
                        <div className="p-2.5 bg-purple-50 text-purple-600 rounded-xl">
                          <ShoppingBag className="w-5 h-5" />
                        </div>
                      </div>
                      <div className="text-[10px] text-amber-600 font-semibold mt-4">
                        {stats.lowStockCount} running low
                      </div>
                    </div>

                    <div className="bg-white border border-gray-200 rounded-2xl p-6 shadow-sm flex flex-col justify-between">
                      <div className="flex justify-between items-start">
                        <div>
                          <span className="text-xs font-bold text-gray-400 uppercase tracking-wider">Unread Inquiries</span>
                          <h2 className="text-2xl font-extrabold text-gray-900 mt-1.5">
                            {stats.pendingMessagesCount}
                          </h2>
                        </div>
                        <div className="p-2.5 bg-red-50 text-red-600 rounded-xl">
                          <MessageSquare className="w-5 h-5" />
                        </div>
                      </div>
                      <div className="text-[10px] text-red-500 font-semibold mt-4">
                        Requires action
                      </div>
                    </div>
                  </div>

                  {/* Dashboard Bottom Section */}
                  <div className="grid lg:grid-cols-3 gap-6">
                    {/* Recent Orders List */}
                    <div className="bg-white border border-gray-200 rounded-2xl p-5 shadow-sm lg:col-span-2">
                      <div className="flex items-center justify-between mb-4">
                        <h3 className="font-bold text-gray-900 text-sm">Recent Orders</h3>
                        <button onClick={() => setActiveTab('orders')} className="text-xs font-bold text-brand-600 hover:underline">
                          View All
                        </button>
                      </div>

                      <div className="divide-y divide-gray-100">
                        {stats.recentOrders.length === 0 ? (
                          <div className="py-8 text-center text-xs text-gray-400">No orders registered.</div>
                        ) : (
                          stats.recentOrders.map(order => (
                            <div key={order.id} className="py-3 flex items-center justify-between text-xs">
                              <div>
                                <p className="font-bold text-gray-800">{order.order_ref}</p>
                                <p className="text-gray-400 mt-0.5">{order.customer_name} • {new Date(order.created_at).toLocaleDateString()}</p>
                              </div>
                              <div className="flex items-center gap-3">
                                <span className={`px-2 py-0.5 rounded-full text-[10px] font-bold ${
                                  order.status === 'delivered' ? 'bg-green-50 text-green-700' :
                                  order.status === 'pending' ? 'bg-amber-50 text-amber-700' :
                                  'bg-blue-50 text-blue-700'
                                }`}>
                                  {order.status}
                                </span>
                                <span className="font-semibold text-gray-700">KSh {order.total_amount.toLocaleString()}</span>
                              </div>
                            </div>
                          ))
                        )}
                      </div>
                    </div>

                    {/* Shop details & Quick Metrics */}
                    <div className="bg-white border border-gray-200 rounded-2xl p-5 shadow-sm">
                      <h3 className="font-bold text-gray-900 text-sm mb-4 font-serif">Quick Actions</h3>
                      <div className="space-y-3">
                        <button 
                          onClick={() => handleOpenProductModal()}
                          className="w-full h-11 bg-brand-600 hover:bg-brand-700 text-white rounded-xl text-xs font-semibold flex items-center justify-center gap-2 transition-colors"
                        >
                          <Plus className="w-4 h-4" />
                          Add New Product
                        </button>
                        <button 
                          onClick={() => handleOpenCategoryModal()}
                          className="w-full h-11 border border-gray-200 hover:bg-gray-50 text-gray-700 rounded-xl text-xs font-semibold flex items-center justify-center gap-2 transition-colors"
                        >
                          <Plus className="w-4 h-4" />
                          Create Category
                        </button>
                        
                        <div className="h-px bg-gray-100 my-4"></div>

                        <div className="space-y-2.5 text-xs">
                          <div className="flex justify-between items-center text-gray-500">
                            <span>M-Pesa Till:</span>
                            <span className="font-bold text-gray-800">4149288 (Buy Goods)</span>
                          </div>
                          <div className="flex justify-between items-center text-gray-500">
                            <span>Total Subscribers:</span>
                            <span className="font-bold text-gray-800">{stats.totalSubscribers}</span>
                          </div>
                          <div className="flex justify-between items-center text-gray-500">
                            <span>Products Featured:</span>
                            <span className="font-bold text-gray-800">{products.filter(p => p.is_featured).length}</span>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              )}

              {/* Tab 2: PRODUCTS */}
              {activeTab === 'products' && (
                <div className="space-y-6">
                  <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
                    <div>
                      <h1 className="font-serif text-2xl lg:text-3xl font-bold text-gray-900">Product Management</h1>
                      <p className="text-sm text-gray-500 mt-1">Manage catalog inventory, pricing, and availability.</p>
                    </div>
                    <button
                      onClick={() => handleOpenProductModal()}
                      className="h-10 px-4 bg-brand-600 hover:bg-brand-700 text-white font-semibold text-xs rounded-xl flex items-center gap-2 transition-colors"
                    >
                      <Plus className="w-4.5 h-4.5" />
                      Add Product
                    </button>
                  </div>

                  {/* Search & Filter bar */}
                  <div className="bg-white border border-gray-200 rounded-2xl p-4 shadow-sm flex flex-col md:flex-row gap-4 items-center justify-between">
                    <div className="relative w-full md:w-80">
                      <Search className="absolute left-3.5 top-3 w-4 h-4 text-gray-400" />
                      <input
                        type="text"
                        placeholder="Search product name or slug..."
                        value={prodSearch}
                        onChange={(e) => setProdSearch(e.target.value)}
                        className="w-full h-10 pl-10 pr-4 border border-gray-200 rounded-xl text-xs focus:border-brand-500 outline-none"
                      />
                    </div>
                    <div className="w-full md:w-auto flex gap-3.5 self-stretch md:self-auto">
                      <select
                        value={prodCategoryFilter}
                        onChange={(e) => setProdCategoryFilter(e.target.value)}
                        className="flex-1 md:w-48 h-10 border border-gray-200 rounded-xl px-3.5 text-xs focus:border-brand-500 outline-none bg-white font-semibold"
                      >
                        <option value="">All Categories</option>
                        {categories.map(cat => (
                          <option key={cat.id} value={cat.id}>{cat.name}</option>
                        ))}
                      </select>
                    </div>
                  </div>

                  {/* Products Table */}
                  <div className="bg-white border border-gray-200 rounded-2xl shadow-sm overflow-hidden">
                    <div className="overflow-x-auto">
                      <table className="w-full text-left border-collapse text-xs">
                        <thead>
                          <tr className="bg-gray-50 border-b border-gray-200 text-gray-500 uppercase tracking-wider font-bold h-11">
                            <th className="px-6">Product details</th>
                            <th className="px-4">Category</th>
                            <th className="px-4">Price</th>
                            <th className="px-4">Stock</th>
                            <th className="px-4">Status</th>
                            <th className="px-6 text-right">Actions</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-gray-100">
                          {filteredProducts.length === 0 ? (
                            <tr>
                              <td colSpan="6" className="py-12 text-center text-gray-400 font-medium">No products match your parameters.</td>
                            </tr>
                          ) : (
                            filteredProducts.map(prod => {
                              const cat = categories.find(c => c.id === prod.category_id);
                              return (
                                <tr key={prod.id} className="hover:bg-gray-50 transition-colors">
                                  <td className="px-6 py-4 flex items-center gap-3.5 min-w-[280px]">
                                    <img 
                                      src={prod.images && prod.images[0] ? prod.images[0] : 'https://images.unsplash.com/photo-1505740420928-5e560c06d30e?w=100'} 
                                      alt="" 
                                      className="w-12 h-12 object-cover rounded-lg border border-gray-100"
                                    />
                                    <div>
                                      <p className="font-bold text-gray-900 text-sm">{prod.name}</p>
                                      <p className="text-[10px] text-gray-400 mt-0.5">{prod.slug}</p>
                                    </div>
                                  </td>
                                  <td className="px-4 py-4 font-semibold text-gray-600">
                                    {cat ? cat.name : 'Unknown'}
                                  </td>
                                  <td className="px-4 py-4 font-bold text-gray-900">
                                    KSh {prod.price.toLocaleString()}
                                  </td>
                                  <td className="px-4 py-4 font-bold text-gray-700">
                                    {prod.stock_quantity}
                                  </td>
                                  <td className="px-4 py-4">
                                    <span className={`px-2 py-0.5 rounded-full text-[10px] font-bold ${
                                      prod.stock_quantity <= 0 ? 'bg-red-50 text-red-700' :
                                      prod.stock_quantity <= 5 ? 'bg-amber-50 text-amber-700' :
                                      'bg-green-50 text-green-700'
                                    }`}>
                                      {prod.stock_quantity <= 0 ? 'Out of stock' :
                                       prod.stock_quantity <= 5 ? 'Low stock' : 'In stock'}
                                    </span>
                                  </td>
                                  <td className="px-6 py-4 text-right">
                                    <div className="flex justify-end gap-2.5">
                                      <button 
                                        onClick={() => handleOpenProductModal(prod)}
                                        className="p-1.5 text-gray-500 hover:text-brand-600 hover:bg-gray-100 rounded-lg transition-all"
                                        title="Edit Product"
                                      >
                                        <Edit className="w-4 h-4" />
                                      </button>
                                      <button 
                                        onClick={() => handleDeleteProduct(prod.id)}
                                        className="p-1.5 text-gray-500 hover:text-red-600 hover:bg-red-50 rounded-lg transition-all"
                                        title="Delete Product"
                                      >
                                        <Trash2 className="w-4 h-4" />
                                      </button>
                                    </div>
                                  </td>
                                </tr>
                              );
                            })
                          )}
                        </tbody>
                      </table>
                    </div>
                  </div>
                </div>
              )}

              {/* Tab 3: CATEGORIES */}
              {activeTab === 'categories' && (
                <div className="space-y-6">
                  <div className="flex justify-between items-center">
                    <div>
                      <h1 className="font-serif text-2xl lg:text-3xl font-bold text-gray-900">Category Catalog</h1>
                      <p className="text-sm text-gray-500 mt-1">Configure shop categories and icons.</p>
                    </div>
                    <button
                      onClick={() => handleOpenCategoryModal()}
                      className="h-10 px-4 bg-brand-600 hover:bg-brand-700 text-white font-semibold text-xs rounded-xl flex items-center gap-2 transition-colors"
                    >
                      <Plus className="w-4.5 h-4.5" />
                      Add Category
                    </button>
                  </div>

                  {/* Grid layout for categories */}
                  <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
                    {categories.map(cat => (
                      <div key={cat.id} className="bg-white border border-gray-200 rounded-2xl p-5 shadow-sm flex items-center justify-between">
                        <div className="flex items-center gap-4">
                          <span className="text-3xl p-2 bg-gray-50 rounded-xl">{cat.icon || '📁'}</span>
                          <div>
                            <h3 className="font-bold text-gray-900 text-base">{cat.name}</h3>
                            <p className="text-xs text-gray-400 mt-0.5">{cat.slug}</p>
                            <span className="inline-block bg-brand-50 text-brand-700 text-[10px] font-bold px-2 py-0.5 rounded-full mt-2">
                              {products.filter(p => p.category_id === cat.id).length} products
                            </span>
                          </div>
                        </div>

                        <div className="flex gap-1.5 self-start">
                          <button 
                            onClick={() => handleOpenCategoryModal(cat)}
                            className="p-1.5 text-gray-400 hover:text-brand-600 hover:bg-gray-50 rounded-lg"
                          >
                            <Edit className="w-4 h-4" />
                          </button>
                          <button 
                            onClick={() => handleDeleteCategory(cat.id)}
                            className="p-1.5 text-gray-400 hover:text-red-600 hover:bg-red-50 rounded-lg"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Tab 4: ORDERS */}
              {activeTab === 'orders' && (
                <div className="space-y-6">
                  <div>
                    <h1 className="font-serif text-2xl lg:text-3xl font-bold text-gray-900">Order Management</h1>
                    <p className="text-sm text-gray-500 mt-1">Review checkout order status, payments, and address logistics.</p>
                  </div>

                  {/* Search and filter toolbar */}
                  <div className="bg-white border border-gray-200 rounded-2xl p-4 shadow-sm flex flex-col md:flex-row gap-4 items-center justify-between">
                    <div className="relative w-full md:w-80">
                      <Search className="absolute left-3.5 top-3 w-4 h-4 text-gray-400" />
                      <input
                        type="text"
                        placeholder="Search ref or customer name..."
                        value={orderSearch}
                        onChange={(e) => setOrderSearch(e.target.value)}
                        className="w-full h-10 pl-10 pr-4 border border-gray-200 rounded-xl text-xs focus:border-brand-500 outline-none"
                      />
                    </div>
                    <div className="w-full md:w-auto flex flex-wrap md:flex-nowrap gap-3 self-stretch md:self-auto">
                      <select
                        value={orderStatusFilter}
                        onChange={(e) => setOrderStatusFilter(e.target.value)}
                        className="flex-1 md:w-36 h-10 border border-gray-200 rounded-xl px-3 text-xs focus:border-brand-500 outline-none bg-white font-semibold"
                      >
                        <option value="">All Orders</option>
                        <option value="pending">Pending</option>
                        <option value="confirmed">Confirmed</option>
                        <option value="shipped">Shipped</option>
                        <option value="delivered">Delivered</option>
                        <option value="cancelled">Cancelled</option>
                      </select>

                      <select
                        value={orderPaymentFilter}
                        onChange={(e) => setOrderPaymentFilter(e.target.value)}
                        className="flex-1 md:w-40 h-10 border border-gray-200 rounded-xl px-3 text-xs focus:border-brand-500 outline-none bg-white font-semibold"
                      >
                        <option value="">All Payments</option>
                        <option value="pending">Payment Pending</option>
                        <option value="paid">Payment Paid</option>
                        <option value="failed">Payment Failed</option>
                      </select>
                    </div>
                  </div>

                  {/* Orders Table */}
                  <div className="bg-white border border-gray-200 rounded-2xl shadow-sm overflow-hidden">
                    <div className="overflow-x-auto">
                      <table className="w-full text-left border-collapse text-xs">
                        <thead>
                          <tr className="bg-gray-50 border-b border-gray-200 text-gray-500 uppercase tracking-wider font-bold h-11">
                            <th className="px-6">Order Ref</th>
                            <th className="px-4">Customer</th>
                            <th className="px-4">Date</th>
                            <th className="px-4">Total</th>
                            <th className="px-4">Order Status</th>
                            <th className="px-4">Payment</th>
                            <th className="px-6 text-right">Details</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-gray-100">
                          {filteredOrders.length === 0 ? (
                            <tr>
                              <td colSpan="7" className="py-12 text-center text-gray-400 font-medium">No orders registered under these parameters.</td>
                            </tr>
                          ) : (
                            filteredOrders.map(order => (
                              <tr key={order.id} className="hover:bg-gray-50 transition-colors">
                                <td className="px-6 py-4 font-bold text-gray-900 text-sm">
                                  {order.order_ref}
                                </td>
                                <td className="px-4 py-4">
                                  <p className="font-bold text-gray-800">{order.customer_name}</p>
                                  <p className="text-[10px] text-gray-400 mt-0.5">{order.customer_phone}</p>
                                </td>
                                <td className="px-4 py-4 text-gray-500">
                                  {new Date(order.created_at).toLocaleDateString()}
                                </td>
                                <td className="px-4 py-4 font-bold text-gray-900">
                                  KSh {order.total_amount.toLocaleString()}
                                </td>
                                <td className="px-4 py-4">
                                  <span className={`px-2.5 py-0.5 rounded-full text-[10px] font-bold capitalize ${
                                    order.status === 'delivered' ? 'bg-green-50 text-green-700' :
                                    order.status === 'pending' ? 'bg-amber-50 text-amber-700' :
                                    order.status === 'cancelled' ? 'bg-red-50 text-red-700' :
                                    'bg-blue-50 text-blue-700'
                                  }`}>
                                    {order.status}
                                  </span>
                                </td>
                                <td className="px-4 py-4">
                                  <span className={`px-2 py-0.5 rounded-full text-[10px] font-bold ${
                                    order.payment_status === 'paid' ? 'bg-green-100 text-green-800' :
                                    order.payment_status === 'failed' ? 'bg-red-100 text-red-800' :
                                    'bg-gray-100 text-gray-600'
                                  }`}>
                                    {order.payment_status === 'paid' ? 'Paid' :
                                     order.payment_status === 'failed' ? 'Failed' : 'Pending'}
                                  </span>
                                </td>
                                <td className="px-6 py-4 text-right">
                                  <button 
                                    onClick={() => setSelectedOrder(order)}
                                    className="p-1.5 border border-gray-200 hover:bg-gray-100 hover:border-gray-300 rounded-lg text-gray-700 font-semibold flex items-center gap-1.5 ml-auto"
                                  >
                                    <Eye className="w-3.5 h-3.5" />
                                    View
                                  </button>
                                </td>
                              </tr>
                            ))
                          )}
                        </tbody>
                      </table>
                    </div>
                  </div>
                </div>
              )}

              {/* Tab 5: REVIEWS */}
              {activeTab === 'reviews' && (
                <div className="space-y-6">
                  <div>
                    <h1 className="font-serif text-2xl lg:text-3xl font-bold text-gray-900">Reviews & Testimonials</h1>
                    <p className="text-sm text-gray-500 mt-1">Moderate customer comments appearing on the store homepage.</p>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
                    {reviews.length === 0 ? (
                      <div className="bg-white border border-gray-200 rounded-2xl p-8 text-center text-xs text-gray-400 md:col-span-2">
                        No reviews in database.
                      </div>
                    ) : (
                      reviews.map(rev => {
                        const prod = products.find(p => p.id === rev.product_id);
                        return (
                          <div key={rev.id} className="bg-white border border-gray-200 rounded-2xl p-5 shadow-sm flex flex-col justify-between space-y-4">
                            <div className="space-y-2">
                              <div className="flex justify-between items-start">
                                <div>
                                  <h3 className="font-bold text-gray-900 text-sm">{rev.name}</h3>
                                  <p className="text-[10px] text-gray-400 mt-0.5">
                                    {new Date(rev.created_at).toLocaleDateString()}
                                  </p>
                                </div>
                                <div className="flex text-amber-400">
                                  {[...Array(5)].map((_, i) => (
                                    <Star 
                                      key={i} 
                                      className={`w-4 h-4 fill-current ${i < rev.rating ? 'text-amber-400' : 'text-gray-200'}`} 
                                    />
                                  ))}
                                </div>
                              </div>

                              <p className="text-xs text-gray-500">
                                Product: <strong className="text-gray-800">{prod ? prod.name : 'Unknown Product'}</strong>
                              </p>

                              <p className="text-xs text-gray-600 italic mt-2">
                                "{rev.comment}"
                              </p>
                            </div>

                            <div className="flex items-center justify-between border-t border-gray-100 pt-3">
                              <span className={`px-2 py-0.5 rounded-full text-[10px] font-bold ${
                                rev.is_approved ? 'bg-green-50 text-green-700' : 'bg-amber-50 text-amber-700'
                              }`}>
                                {rev.is_approved ? 'Approved' : 'Pending Review'}
                              </span>

                              <div className="flex gap-2">
                                {!rev.is_approved && (
                                  <button
                                    onClick={() => handleApproveReview(rev.id)}
                                    className="h-8 px-3.5 bg-green-600 hover:bg-green-700 text-white rounded-lg text-[10px] font-bold flex items-center gap-1"
                                  >
                                    <Check className="w-3.5 h-3.5" />
                                    Approve
                                  </button>
                                )}
                                <button
                                  onClick={() => handleRejectReview(rev.id)}
                                  className="h-8 px-3.5 border border-red-200 hover:bg-red-50 text-red-600 rounded-lg text-[10px] font-bold flex items-center gap-1"
                                >
                                  <Trash2 className="w-3.5 h-3.5" />
                                  Delete
                                </button>
                              </div>
                            </div>
                          </div>
                        );
                      })
                    )}
                  </div>
                </div>
              )}

              {/* Tab 6: INQUIRIES */}
              {activeTab === 'messages' && (
                <div className="space-y-6">
                  <div>
                    <h1 className="font-serif text-2xl lg:text-3xl font-bold text-gray-900">Customer Inquiries</h1>
                    <p className="text-sm text-gray-500 mt-1">Review contact inquiries left by store visitors.</p>
                  </div>

                  <div className="space-y-4">
                    {messages.length === 0 ? (
                      <div className="bg-white border border-gray-200 rounded-2xl p-8 text-center text-xs text-gray-400">
                        No customer messages found.
                      </div>
                    ) : (
                      messages.map(msg => (
                        <div key={msg.id} className={`bg-white border rounded-2xl p-5 shadow-sm space-y-3 ${
                          !msg.is_read ? 'border-brand-500 ring-1 ring-brand-500' : 'border-gray-200'
                        }`}>
                          <div className="flex flex-col sm:flex-row justify-between items-start gap-2">
                            <div>
                              <h3 className="font-bold text-gray-900 text-base">{msg.name}</h3>
                              <p className="text-xs text-gray-500 mt-1 flex flex-wrap gap-x-3 gap-y-1">
                                <span>📞 {msg.phone || 'No phone'}</span>
                                <span>✉️ {msg.email || 'No email'}</span>
                                <span>🕒 {new Date(msg.created_at).toLocaleString()}</span>
                              </p>
                            </div>
                            <div className="flex gap-2">
                              <button
                                onClick={() => handleToggleMessageRead(msg)}
                                className={`h-8 px-3.5 rounded-lg text-[10px] font-bold ${
                                  msg.is_read 
                                    ? 'border border-gray-200 hover:bg-gray-50 text-gray-600' 
                                    : 'bg-brand-600 hover:bg-brand-700 text-white'
                                }`}
                              >
                                {msg.is_read ? 'Mark Unread' : 'Mark Read'}
                              </button>
                            </div>
                          </div>

                          <div className="bg-gray-50 rounded-xl p-3.5 border border-gray-100 text-xs text-gray-700 whitespace-pre-wrap">
                            {msg.message}
                          </div>
                        </div>
                      ))
                    )}
                  </div>
                </div>
              )}

              {/* Tab 7: SUBSCRIBERS */}
              {activeTab === 'subscribers' && (
                <div className="space-y-6">
                  <div>
                    <h1 className="font-serif text-2xl lg:text-3xl font-bold text-gray-900">Newsletter Subscribers</h1>
                    <p className="text-sm text-gray-500 mt-1">E-Commerce newsletter subscribers for email marketing campaigns.</p>
                  </div>

                  <div className="bg-white border border-gray-200 rounded-2xl shadow-sm overflow-hidden">
                    <div className="overflow-x-auto">
                      <table className="w-full text-left border-collapse text-xs">
                        <thead>
                          <tr className="bg-gray-50 border-b border-gray-200 text-gray-500 uppercase tracking-wider font-bold h-11">
                            <th className="px-6">Email Address</th>
                            <th className="px-4">Subscriber Name</th>
                            <th className="px-4">Date Joined</th>
                            <th className="px-4">Status</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-gray-100">
                          {subscribers.length === 0 ? (
                            <tr>
                              <td colSpan="4" className="py-12 text-center text-gray-400 font-medium">No subscribers registered.</td>
                            </tr>
                          ) : (
                            subscribers.map(sub => (
                              <tr key={sub.id} className="hover:bg-gray-50 transition-colors">
                                <td className="px-6 py-4 font-bold text-gray-900 text-sm">
                                  {sub.email}
                                </td>
                                <td className="px-4 py-4 text-gray-600 font-semibold">
                                  {sub.name || 'Subscriber'}
                                </td>
                                <td className="px-4 py-4 text-gray-500">
                                  {sub.subscribed_at ? new Date(sub.subscribed_at).toLocaleDateString() : 'N/A'}
                                </td>
                                <td className="px-4 py-4">
                                  <span className={`px-2 py-0.5 rounded-full text-[10px] font-bold ${
                                    sub.is_active !== false ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500'
                                  }`}>
                                    {sub.is_active !== false ? 'Active' : 'Unsubscribed'}
                                  </span>
                                </td>
                              </tr>
                            ))
                          )}
                        </tbody>
                      </table>
                    </div>
                  </div>
                </div>
              )}
            </>
          )}
        </main>
      </div>

      {/* ======================================================== */}
      {/* 1. ORDER DETAILS MODAL */}
      {/* ======================================================== */}
      {selectedOrder && (
        <div className="fixed inset-0 z-50 overflow-y-auto bg-black/50 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl w-full max-w-2xl overflow-hidden shadow-xl max-h-[90vh] flex flex-col">
            
            {/* Header */}
            <div className="px-6 py-4 border-b border-gray-100 flex items-center justify-between sticky top-0 bg-white">
              <div>
                <h3 className="font-bold text-gray-900 text-base flex items-center gap-2">
                  Order Details: {selectedOrder.order_ref}
                </h3>
                <p className="text-[10px] text-gray-400 mt-0.5">Placed on {new Date(selectedOrder.created_at).toLocaleString()}</p>
              </div>
              <button 
                onClick={() => setSelectedOrder(null)} 
                className="p-1 hover:bg-gray-100 rounded-lg text-gray-500"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {/* Content */}
            <div className="p-6 overflow-y-auto space-y-6 text-xs flex-1">
              
              {/* Order Status Control */}
              <div className="grid grid-cols-2 gap-4 bg-gray-50 border border-gray-100 rounded-xl p-4">
                <div>
                  <label className="block text-[10px] font-bold text-gray-400 uppercase mb-1.5">Fulfillment Status</label>
                  <select
                    value={selectedOrder.status}
                    onChange={(e) => handleUpdateOrderStatus(selectedOrder.id, e.target.value)}
                    className="w-full h-9 border border-gray-200 rounded-lg px-2 text-xs focus:border-brand-500 outline-none bg-white font-semibold"
                  >
                    <option value="pending">Pending</option>
                    <option value="confirmed">Confirmed</option>
                    <option value="shipped">Shipped</option>
                    <option value="delivered">Delivered</option>
                    <option value="cancelled">Cancelled</option>
                  </select>
                </div>
                <div>
                  <label className="block text-[10px] font-bold text-gray-400 uppercase mb-1.5">Payment Status</label>
                  <select
                    value={selectedOrder.payment_status}
                    onChange={(e) => handleUpdateOrderPaymentStatus(selectedOrder.id, e.target.value)}
                    className="w-full h-9 border border-gray-200 rounded-lg px-2 text-xs focus:border-brand-500 outline-none bg-white font-semibold"
                  >
                    <option value="pending">Pending</option>
                    <option value="paid">Paid</option>
                    <option value="failed">Failed</option>
                  </select>
                </div>
              </div>

              {/* Customer details */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div className="space-y-1.5">
                  <h4 className="font-bold text-gray-900">Customer Info</h4>
                  <p className="text-gray-600"><span className="font-medium text-gray-400">Name:</span> {selectedOrder.customer_name}</p>
                  <p className="text-gray-600"><span className="font-medium text-gray-400">Phone:</span> {selectedOrder.customer_phone}</p>
                  <p className="text-gray-600"><span className="font-medium text-gray-400">Email:</span> {selectedOrder.customer_email || 'N/A'}</p>
                </div>
                <div className="space-y-1.5">
                  <h4 className="font-bold text-gray-900">Shipping Info</h4>
                  <p className="text-gray-600"><span className="font-medium text-gray-400">Address:</span> {selectedOrder.shipping_address || 'Shop Pickup'}</p>
                  <p className="text-gray-600"><span className="font-medium text-gray-400">Fulfillment:</span> {selectedOrder.shipping_address ? 'Home Delivery' : 'Self Pickup'}</p>
                  {selectedOrder.mpesa_transaction_id && (
                    <p className="text-green-600 font-semibold"><span className="text-gray-400 font-medium">M-Pesa Txn ID:</span> {selectedOrder.mpesa_transaction_id}</p>
                  )}
                </div>
              </div>

              {selectedOrder.notes && (
                <div className="space-y-1">
                  <h4 className="font-bold text-gray-900">Customer Note</h4>
                  <p className="text-gray-600 bg-gray-50 border border-gray-100 rounded-lg p-3 italic">"{selectedOrder.notes}"</p>
                </div>
              )}

              {/* Order Items */}
              <div className="space-y-2">
                <h4 className="font-bold text-gray-900">Order Items</h4>
                <div className="border border-gray-100 rounded-xl overflow-hidden">
                  <div className="bg-gray-50 border-b border-gray-100 h-9 px-4 flex items-center font-bold text-gray-500 uppercase tracking-wider">
                    <span className="flex-1">Item</span>
                    <span className="w-16 text-center">Qty</span>
                    <span className="w-24 text-right">Price</span>
                    <span className="w-24 text-right">Total</span>
                  </div>
                  <div className="divide-y divide-gray-100">
                    {selectedOrder.items.map((item, idx) => (
                      <div key={idx} className="px-4 py-3 flex items-center">
                        <span className="flex-1 font-bold text-gray-800">{item.product_name}</span>
                        <span className="w-16 text-center font-medium text-gray-600">{item.quantity}</span>
                        <span className="w-24 text-right font-medium text-gray-600">KSh {item.unit_price.toLocaleString()}</span>
                        <span className="w-24 text-right font-bold text-gray-900">KSh {item.total_price.toLocaleString()}</span>
                      </div>
                    ))}
                  </div>
                  <div className="bg-gray-50 border-t border-gray-100 p-4 space-y-1.5 text-right font-semibold">
                    <div className="text-gray-500 text-[10px]">
                      Delivery Fee: <span className="text-gray-800">KSh {selectedOrder.total_amount >= 600 ? 0 : 150}</span>
                    </div>
                    <div className="text-sm font-bold text-gray-900">
                      Total Amount: KSh {selectedOrder.total_amount.toLocaleString()}
                    </div>
                  </div>
                </div>
              </div>

            </div>

            {/* Footer */}
            <div className="px-6 py-4 border-t border-gray-100 flex justify-end sticky bottom-0 bg-white">
              <button 
                onClick={() => setSelectedOrder(null)} 
                className="h-10 px-5 bg-gray-100 hover:bg-gray-200 text-gray-700 rounded-xl font-bold transition-all"
              >
                Close Panel
              </button>
            </div>

          </div>
        </div>
      )}

      {/* ======================================================== */}
      {/* 2. ADD/EDIT PRODUCT MODAL */}
      {/* ======================================================== */}
      {isProductModalOpen && (
        <div className="fixed inset-0 z-50 overflow-y-auto bg-black/50 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl w-full max-w-lg overflow-hidden shadow-xl max-h-[95vh] flex flex-col">
            
            <div className="px-6 py-4 border-b border-gray-100 flex items-center justify-between sticky top-0 bg-white">
              <h3 className="font-serif text-lg font-bold text-gray-900">
                {editingProduct ? 'Edit Product Details' : 'Add New Product to Catalog'}
              </h3>
              <button 
                onClick={() => setIsProductModalOpen(false)} 
                className="p-1 hover:bg-gray-100 rounded-lg text-gray-500"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <form onSubmit={handleProductSubmit} className="flex-1 overflow-y-auto p-6 space-y-4 text-xs">
              <div className="grid grid-cols-2 gap-4">
                <div className="col-span-2">
                  <label className="block text-[10px] font-bold text-gray-500 uppercase mb-1.5">Product Name</label>
                  <input
                    type="text"
                    required
                    value={productForm.name}
                    onChange={(e) => {
                      const name = e.target.value;
                      const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, '-');
                      setProductForm({ ...productForm, name, slug });
                    }}
                    placeholder="e.g. MK 2-Way Light Switch"
                    className="w-full h-11 border border-gray-200 rounded-xl px-3.5 text-xs focus:border-brand-500 outline-none"
                  />
                </div>
                
                <div>
                  <label className="block text-[10px] font-bold text-gray-500 uppercase mb-1.5">Price (KSh)</label>
                  <input
                    type="number"
                    step="0.01"
                    required
                    value={productForm.price}
                    onChange={(e) => setProductForm({ ...productForm, price: e.target.value })}
                    placeholder="e.g. 250"
                    className="w-full h-11 border border-gray-200 rounded-xl px-3.5 text-xs focus:border-brand-500 outline-none"
                  />
                </div>

                <div>
                  <label className="block text-[10px] font-bold text-gray-500 uppercase mb-1.5">Stock Quantity</label>
                  <input
                    type="number"
                    required
                    value={productForm.stock_quantity}
                    onChange={(e) => setProductForm({ ...productForm, stock_quantity: e.target.value })}
                    placeholder="e.g. 50"
                    className="w-full h-11 border border-gray-200 rounded-xl px-3.5 text-xs focus:border-brand-500 outline-none"
                  />
                </div>

                <div>
                  <label className="block text-[10px] font-bold text-gray-500 uppercase mb-1.5">Category</label>
                  <select
                    value={productForm.category_id}
                    required
                    onChange={(e) => setProductForm({ ...productForm, category_id: e.target.value })}
                    className="w-full h-11 border border-gray-200 rounded-xl px-3 text-xs focus:border-brand-500 outline-none bg-white font-semibold"
                  >
                    {categories.map(cat => (
                      <option key={cat.id} value={cat.id}>{cat.name}</option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="block text-[10px] font-bold text-gray-500 uppercase mb-1.5">Subcategory</label>
                  <input
                    type="text"
                    value={productForm.subcategory}
                    onChange={(e) => setProductForm({ ...productForm, subcategory: e.target.value })}
                    placeholder="e.g. Switches"
                    className="w-full h-11 border border-gray-200 rounded-xl px-3.5 text-xs focus:border-brand-500 outline-none"
                  />
                </div>

                <div className="col-span-2">
                  <label className="block text-[10px] font-bold text-gray-500 uppercase mb-1.5">Image URL</label>
                  <input
                    type="url"
                    value={productForm.image_url}
                    onChange={(e) => setProductForm({ ...productForm, image_url: e.target.value })}
                    placeholder="https://images.unsplash.com/photo-..."
                    className="w-full h-11 border border-gray-200 rounded-xl px-3.5 text-xs focus:border-brand-500 outline-none"
                  />
                </div>

                <div className="col-span-2">
                  <label className="block text-[10px] font-bold text-gray-500 uppercase mb-1.5">Product Description</label>
                  <textarea
                    rows={4}
                    value={productForm.description}
                    onChange={(e) => setProductForm({ ...productForm, description: e.target.value })}
                    placeholder="Detail the specifications, package content, warranty terms..."
                    className="w-full border border-gray-200 rounded-xl p-3 text-xs focus:border-brand-500 outline-none resize-none"
                  />
                </div>
              </div>

              <div className="border-t border-gray-100 pt-4 flex justify-end gap-3 sticky bottom-0 bg-white">
                <button
                  type="button"
                  onClick={() => setIsProductModalOpen(false)}
                  className="h-11 px-5 border border-gray-200 hover:bg-gray-50 text-gray-700 rounded-xl font-bold transition-all"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="h-11 px-5 bg-brand-600 hover:bg-brand-700 text-white rounded-xl font-bold transition-all"
                >
                  {editingProduct ? 'Save Product' : 'Create Product'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* ======================================================== */}
      {/* 3. ADD/EDIT CATEGORY MODAL */}
      {/* ======================================================== */}
      {isCategoryModalOpen && (
        <div className="fixed inset-0 z-50 overflow-y-auto bg-black/50 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl w-full max-w-sm overflow-hidden shadow-xl flex flex-col">
            
            <div className="px-6 py-4 border-b border-gray-100 flex items-center justify-between">
              <h3 className="font-serif text-lg font-bold text-gray-900">
                {editingCategory ? 'Edit Category' : 'Add Category'}
              </h3>
              <button 
                onClick={() => setIsCategoryModalOpen(false)} 
                className="p-1 hover:bg-gray-100 rounded-lg text-gray-500"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <form onSubmit={handleCategorySubmit} className="p-6 space-y-4 text-xs">
              <div>
                <label className="block text-[10px] font-bold text-gray-500 uppercase mb-1.5">Category Name</label>
                <input
                  type="text"
                  required
                  value={categoryForm.name}
                  onChange={(e) => {
                    const name = e.target.value;
                    const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, '-');
                    setCategoryForm({ ...categoryForm, name, slug });
                  }}
                  placeholder="e.g. Solar Equipment"
                  className="w-full h-11 border border-gray-200 rounded-xl px-3.5 text-xs focus:border-brand-500 outline-none"
                />
              </div>

              <div>
                <label className="block text-[10px] font-bold text-gray-500 uppercase mb-1.5">Category Icon Emoji</label>
                <input
                  type="text"
                  required
                  value={categoryForm.icon}
                  onChange={(e) => setCategoryForm({ ...categoryForm, icon: e.target.value })}
                  placeholder="e.g. ☀️"
                  className="w-full h-11 border border-gray-200 rounded-xl px-3.5 text-xs focus:border-brand-500 outline-none"
                />
              </div>

              <div className="border-t border-gray-100 pt-4 flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setIsCategoryModalOpen(false)}
                  className="h-11 px-5 border border-gray-200 hover:bg-gray-50 text-gray-700 rounded-xl font-bold transition-all"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="h-11 px-5 bg-brand-600 hover:bg-brand-700 text-white rounded-xl font-bold transition-all"
                >
                  {editingCategory ? 'Save Changes' : 'Create Category'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

    </div>
  );
}
