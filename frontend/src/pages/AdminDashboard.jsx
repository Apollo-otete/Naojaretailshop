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
  Menu,
  Upload,
  ImageIcon,
  LogOut,
  Key,
  Lock,
  Activity,
  Sparkles,
  CreditCard,
  Truck,
  BellRing,
  PackageSearch,
  Radio
} from 'lucide-react';
import { analyticsApi } from '../lib/analyticsApi';
import { SSEClient } from '../lib/sse';
import LiveOverviewTab from '../components/admin/LiveOverviewTab';
import SalesAnalyticsTab from '../components/admin/SalesAnalyticsTab';
import InventoryIntelligenceTab from '../components/admin/InventoryIntelligenceTab';
import ForecastPlanningTab from '../components/admin/ForecastPlanningTab';
import PaymentHealthTab from '../components/admin/PaymentHealthTab';
import FulfillmentTab from '../components/admin/FulfillmentTab';
import AlertsCentreTab from '../components/admin/AlertsCentreTab';

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

  // Business Intelligence & Live Monitor States
  const [salesRange, setSalesRange] = useState('week');
  const [paymentRange, setPaymentRange] = useState('month');
  const [fulfillmentRange, setFulfillmentRange] = useState('month');
  const [soundEnabled, setSoundEnabled] = useState(true);

  const [liveAnalytics, setLiveAnalytics] = useState(null);
  const [liveEvents, setLiveEvents] = useState([]);
  const [salesSummary, setSalesSummary] = useState(null);
  const [categoryAnalytics, setCategoryAnalytics] = useState([]);
  const [inventoryHealth, setInventoryHealth] = useState([]);
  const [forecastData, setForecastData] = useState(null);
  const [insightsData, setInsightsData] = useState([]);
  const [targetData, setTargetData] = useState(null);
  const [paymentData, setPaymentData] = useState(null);
  const [fulfillmentData, setFulfillmentData] = useState(null);
  const [alertsData, setAlertsData] = useState([]);
  const [analyticsLoading, setAnalyticsLoading] = useState(false);
  const [products, setProducts] = useState([]);
  const [categories, setCategories] = useState([]);
  const [orders, setOrders] = useState([]);
  const [reviews, setReviews] = useState([]);
  const [subscribers, setSubscribers] = useState([]);
  const [messages, setMessages] = useState([]);
  
  const [loading, setLoading] = useState(true);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [uploadingImage, setUploadingImage] = useState(false);

  // Search & Filter States
  const [prodSearch, setProdSearch] = useState('');
  const [prodCategoryFilter, setProdCategoryFilter] = useState('');
  const [prodPage, setProdPage] = useState(1);
  const [orderSearch, setOrderSearch] = useState('');
  const [orderStatusFilter, setOrderStatusFilter] = useState('');
  const [orderPaymentFilter, setOrderPaymentFilter] = useState('');
  const [orderPage, setOrderPage] = useState(1);
  const adminPageSize = 10;

  // Modals States
  const [selectedOrder, setSelectedOrder] = useState(null);
  const [isProductModalOpen, setIsProductModalOpen] = useState(false);
  const [editingProduct, setEditingProduct] = useState(null);
  const [isCategoryModalOpen, setIsCategoryModalOpen] = useState(false);
  const [editingCategory, setEditingCategory] = useState(null);
  const [isPasswordModalOpen, setIsPasswordModalOpen] = useState(false);
  const [passwordForm, setPasswordForm] = useState({ currentPassword: '', newPassword: '', confirmPassword: '' });
  const [passwordStatus, setPasswordStatus] = useState({ loading: false, error: '', success: '' });

  // Form States - Product
  const [productForm, setProductForm] = useState({
    name: '',
    slug: '',
    description: '',
    price: '',
    stock_quantity: '',
    category_id: '',
    subcategory: '',
    image_url: '',
    image_preview: ''
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

  // Web Audio Synthesizer Chime for Live Daraja M-Pesa notifications
  const playChime = () => {
    try {
      const AudioCtx = window.AudioContext || window.webkitAudioContext;
      if (!AudioCtx) return;
      const ctx = new AudioCtx();
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = 'sine';
      osc.frequency.setValueAtTime(587.33, ctx.currentTime);
      osc.frequency.exponentialRampToValueAtTime(880, ctx.currentTime + 0.15);
      gain.gain.setValueAtTime(0.12, ctx.currentTime);
      gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.35);
      osc.connect(gain);
      gain.connect(ctx.destination);
      osc.start();
      osc.stop(ctx.currentTime + 0.35);
    } catch (e) {
      // Audio autoplay policy
    }
  };

  const fetchAllAnalytics = async () => {
    setAnalyticsLoading(true);
    try {
      const [
        liveRes,
        summaryRes,
        catRes,
        invRes,
        fcRes,
        insRes,
        tgtRes,
        payRes,
        fulRes,
        altRes
      ] = await Promise.all([
        analyticsApi.getLive(),
        analyticsApi.getSummary(salesRange),
        analyticsApi.getCategoryPerformance(salesRange),
        analyticsApi.getInventoryHealth(),
        analyticsApi.getRevenueForecast(30),
        analyticsApi.getInsights(),
        analyticsApi.getTargetProgress('month'),
        analyticsApi.getPaymentHealth(paymentRange),
        analyticsApi.getFulfillment(fulfillmentRange),
        analyticsApi.getAlerts()
      ]);

      if (liveRes) {
        setLiveAnalytics(liveRes);
        if (liveRes.recentEvents && liveRes.recentEvents.length > 0) {
          setLiveEvents(prev => prev.length === 0 ? liveRes.recentEvents : prev);
        }
      }
      if (summaryRes) setSalesSummary(summaryRes);
      if (catRes) setCategoryAnalytics(catRes);
      if (invRes) setInventoryHealth(invRes);
      if (fcRes) setForecastData(fcRes);
      if (insRes) setInsightsData(insRes);
      if (tgtRes) setTargetData(tgtRes);
      if (payRes) setPaymentData(payRes);
      if (fulRes) setFulfillmentData(fulRes);
      if (altRes) setAlertsData(altRes);
    } catch (err) {
      console.warn('Analytics loading error (fallback handled):', err);
    } finally {
      setAnalyticsLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    fetchAllAnalytics();
  }, []);

  // Real-time SSE stream hook
  useEffect(() => {
    const streamUrl = analyticsApi.getStreamUrl();
    const sse = new SSEClient(
      streamUrl,
      (evt) => {
        setLiveEvents((prev) => [
          {
            id: evt.id || `ev-${Date.now()}`,
            type: evt.type || 'system',
            message: evt.message || (evt.data ? JSON.stringify(evt.data) : 'Customer activity'),
            time: 'Just now',
            amount: evt.amount || null,
            orderRef: evt.orderRef || evt.order_ref || null
          },
          ...prev.slice(0, 29)
        ]);

        analyticsApi.getLive().then(setLiveAnalytics).catch(() => {});
        if (soundEnabled && (evt.type === 'payment_received' || evt.type === 'order_created')) {
          playChime();
        }
      },
      (err) => {
        // SSE reconnect handled internally
      }
    );

    sse.connect();
    return () => {
      sse.disconnect();
    };
  }, [soundEnabled]);

  useEffect(() => {
    analyticsApi.getSummary(salesRange).then(setSalesSummary).catch(() => {});
    analyticsApi.getCategoryPerformance(salesRange).then(setCategoryAnalytics).catch(() => {});
  }, [salesRange]);

  useEffect(() => {
    analyticsApi.getPaymentHealth(paymentRange).then(setPaymentData).catch(() => {});
  }, [paymentRange]);

  useEffect(() => {
    analyticsApi.getFulfillment(fulfillmentRange).then(setFulfillmentData).catch(() => {});
  }, [fulfillmentRange]);

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
        image_url: prod.images && prod.images[0] ? prod.images[0] : '',
        image_preview: prod.images && prod.images[0] ? prod.images[0] : ''
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
        image_url: '',
        image_preview: ''
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
    refreshAnalytics(products, orders, subscribers, updatedMessages);
    // Persist to real API (mark as read only — toggle back is local)
    try {
      if (!msg.is_read) {
        await api.admin.markMessageRead(msg.id);
      }
    } catch (err) {
      console.warn('Mark read API error:', err.message);
    }
    // Also update localStorage fallback
    const db = JSON.parse(localStorage.getItem('naoja_db'));
    if (db) {
      db.messages = db.messages.map(m => m.id === msg.id ? { ...m, is_read: !msg.is_read } : m);
      localStorage.setItem('naoja_db', JSON.stringify(db));
    }
  };

  // Handle password change
  const handlePasswordSubmit = async (e) => {
    e.preventDefault();
    setPasswordStatus({ loading: true, error: '', success: '' });

    if (passwordForm.newPassword !== passwordForm.confirmPassword) {
      setPasswordStatus({ loading: false, error: 'New passwords do not match', success: '' });
      return;
    }

    if (passwordForm.newPassword.length < 6) {
      setPasswordStatus({ loading: false, error: 'Password must be at least 6 characters long', success: '' });
      return;
    }

    try {
      await api.admin.changePassword({
        currentPassword: passwordForm.currentPassword,
        newPassword: passwordForm.newPassword
      });
      setPasswordStatus({ loading: false, error: '', success: 'Password updated successfully!' });
      setTimeout(() => {
        setIsPasswordModalOpen(false);
        setPasswordForm({ currentPassword: '', newPassword: '', confirmPassword: '' });
        setPasswordStatus({ loading: false, error: '', success: '' });
      }, 1500);
    } catch (err) {
      setPasswordStatus({ loading: false, error: err.message || 'Failed to update password', success: '' });
    }
  };

  // Handle logout
  const handleLogout = () => {
    sessionStorage.removeItem('adminToken');
    sessionStorage.removeItem('adminInfo');
    localStorage.removeItem('adminToken');
    localStorage.removeItem('adminInfo');
    window.location.href = '/admin/login';
  };

  // Return to Storefront (Lock session so returning requires re-entering credentials)
  const handleBackToStore = () => {
    sessionStorage.removeItem('adminToken');
    sessionStorage.removeItem('adminInfo');
    localStorage.removeItem('adminToken');
    localStorage.removeItem('adminInfo');
    window.location.href = '/';
  };

  // Handle image file upload
  const handleImageFileChange = async (e) => {
    const file = e.target.files[0];
    if (!file) return;
    // Show local preview immediately
    const localPreview = URL.createObjectURL(file);
    setProductForm(prev => ({ ...prev, image_preview: localPreview }));
    setUploadingImage(true);
    try {
      const res = await api.uploadImage(file);
      setProductForm(prev => ({ ...prev, image_url: res.url, image_preview: res.url }));
    } catch (err) {
      // Backend offline – keep local preview URL as fallback
      setProductForm(prev => ({ ...prev, image_url: localPreview }));
      console.warn('Upload to server failed, using local blob URL as fallback.');
    } finally {
      setUploadingImage(false);
    }
  };

  const unacknowledgedAlertsCount = alertsData.filter(a => !a.acknowledged && (a.severity === 'critical' || a.severity === 'warning')).length;

  const biMenuItems = [
    { id: 'overview', label: 'Live Command', icon: Activity, liveDot: true },
    { id: 'sales_analytics', label: 'Sales & Revenue', icon: TrendingUp },
    { id: 'forecast', label: 'Predictive Forecast', icon: Sparkles },
    { id: 'inventory_intel', label: 'Stock Intelligence', icon: PackageSearch, badge: stats.lowStockCount ? `${stats.lowStockCount} low` : null, badgeColor: 'bg-amber-500' },
    { id: 'payment_health', label: 'M-Pesa Diagnostics', icon: CreditCard },
    { id: 'fulfillment', label: 'Logistics Funnel', icon: Truck },
    { id: 'alerts', label: 'Alerts Centre', icon: BellRing, badge: unacknowledgedAlertsCount ? unacknowledgedAlertsCount : null, badgeColor: 'bg-rose-600' }
  ];

  const storeMenuItems = [
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
    <div className="min-h-screen flex flex-col font-sans" style={{background: 'linear-gradient(135deg, #f0f4ff 0%, #f8f9fc 50%, #f0f7f4 100%)'}}>
      
      {/* Top Banner Navigation */}
      <header className="bg-white/95 backdrop-blur-md border-b border-gray-200/80 h-16 shrink-0 flex items-center justify-between px-4 lg:px-6 sticky top-0 z-30 shadow-xs">
        <div className="flex items-center gap-3">
          <button 
            onClick={() => setSidebarOpen(!sidebarOpen)}
            className="p-2 hover:bg-gray-100 rounded-xl lg:hidden text-gray-600 transition-colors"
            aria-label="Toggle navigation menu"
          >
            <Menu className="w-5 h-5" />
          </button>
          
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-xl bg-gradient-to-tr from-brand-600 to-[#ff6b00] flex items-center justify-center text-white font-display font-black text-lg shadow-sm">
              N
            </div>
            <div className="flex flex-col">
              <span className="font-display text-base font-extrabold text-gray-950 tracking-tight flex items-center gap-1.5 leading-tight">
                NAOJA <span className="text-[10px] font-extrabold uppercase bg-brand-500 text-white px-1.5 py-0.5 rounded tracking-normal">Ops Hub</span>
              </span>
              <span className="text-[10px] uppercase font-bold tracking-wider text-gray-400 -mt-0.5">
                Executive & Retail Control
              </span>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-3 sm:gap-4">
          <div className="hidden sm:flex items-center gap-2 px-3 py-1 bg-emerald-50 border border-emerald-200/80 rounded-full text-xs font-bold text-emerald-800 shadow-xs">
            <span className="relative flex h-2 w-2">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
              <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
            </span>
            <span>Daraja SSE Live Stream</span>
          </div>

          <button
            onClick={handleBackToStore}
            className="text-xs font-bold text-gray-600 hover:text-brand-600 bg-gray-50 hover:bg-brand-50/60 border border-gray-200/80 hover:border-brand-200 px-3.5 py-1.5 rounded-xl flex items-center gap-1.5 transition-all shadow-xs"
            title="Lock session & return to storefront"
          >
            <ArrowLeft className="w-3.5 h-3.5" />
            <span>Storefront</span>
          </button>
        </div>
      </header>

      <div className="flex flex-1 relative overflow-hidden">
        <aside 
          className={`border-r border-gray-200/60 w-64 shrink-0 flex flex-col justify-between py-5 fixed inset-y-0 left-0 z-50 transform lg:static lg:translate-x-0 transition-transform duration-300 shadow-2xl lg:shadow-none ${
            sidebarOpen ? 'translate-x-0' : '-translate-x-full'
          }`} 
          style={{
            background: 'linear-gradient(180deg, #ffffff 0%, #fafbff 100%)',
            paddingTop: 'max(1.25rem, env(safe-area-inset-top))',
            paddingBottom: 'max(1.25rem, env(safe-area-inset-bottom))'
          }}
        >
          <div className="space-y-5 px-3 overflow-y-auto">
            {/* Group 1: Executive Analytics */}
            <div>
              <div className="px-3 pb-2 flex items-center justify-between">
                <span className="text-[9px] font-extrabold tracking-widest uppercase" style={{color: '#3B82F6'}}>Executive Analytics</span>
                <span className="relative flex h-2 w-2">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                  <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                </span>
              </div>
              <div className="space-y-0.5">
                {biMenuItems.map((item) => {
                  const Icon = item.icon;
                  const isActive = activeTab === item.id;
                  return (
                    <button
                      key={item.id}
                      onClick={() => {
                        setActiveTab(item.id);
                        setSidebarOpen(false);
                      }}
                      className={`w-full h-10 px-3.5 rounded-xl flex items-center justify-between font-semibold text-xs transition-all ${
                        isActive
                          ? 'text-white shadow-md'
                          : 'text-gray-600 hover:bg-blue-50/60 hover:text-brand-700'
                      }`}
                      style={isActive ? {background: 'linear-gradient(135deg, #1D4ED8 0%, #3B82F6 100%)'} : {}}
                    >
                      <div className="flex items-center gap-2.5">
                        <Icon className={`w-4 h-4 ${isActive ? 'text-white' : 'text-gray-400'}`} />
                        <span className={isActive ? 'font-bold' : ''}>{item.label}</span>
                      </div>
                      {item.liveDot && !item.badge && (
                        <span className="flex h-2 w-2 relative">
                          <span className={`animate-ping absolute inline-flex h-full w-full rounded-full ${isActive ? 'bg-white' : 'bg-emerald-400'} opacity-75`}></span>
                          <span className={`relative inline-flex rounded-full h-2 w-2 ${isActive ? 'bg-white' : 'bg-emerald-500'}`}></span>
                        </span>
                      )}
                      {item.badge && (
                        <span className={`${isActive ? 'bg-white/20 text-white' : (item.badgeColor || 'bg-brand-600') + ' text-white'} text-[10px] font-extrabold px-1.5 py-0.5 rounded-full`}>
                          {item.badge}
                        </span>
                      )}
                    </button>
                  );
                })}
              </div>
            </div>

            {/* Divider */}
            <div className="border-t border-gray-100 mx-3"></div>

            {/* Group 2: Store Operations */}
            <div>
              <div className="px-3 pb-2">
                <span className="text-[9px] font-extrabold tracking-widest uppercase text-gray-400">Store Operations</span>
              </div>
              <div className="space-y-0.5">
                {storeMenuItems.map((item) => {
                  const Icon = item.icon;
                  const isActive = activeTab === item.id;
                  return (
                    <button
                      key={item.id}
                      onClick={() => {
                        setActiveTab(item.id);
                        setSidebarOpen(false);
                      }}
                      className={`w-full h-10 px-3.5 rounded-xl flex items-center justify-between font-semibold text-xs transition-all ${
                        isActive
                          ? 'text-white shadow-md'
                          : 'text-gray-600 hover:bg-orange-50/60 hover:text-orange-700'
                      }`}
                      style={isActive ? {background: 'linear-gradient(135deg, #ea580c 0%, #ff6b00 100%)'} : {}}
                    >
                      <div className="flex items-center gap-2.5">
                        <Icon className={`w-4 h-4 ${isActive ? 'text-white' : 'text-gray-400'}`} />
                        <span className={isActive ? 'font-bold' : ''}>{item.label}</span>
                      </div>
                      {item.badge && (
                        <span className={`${isActive ? 'bg-white/25 text-white' : 'bg-orange-500 text-white'} text-[10px] font-extrabold px-1.5 py-0.5 rounded-full`}>
                          {item.badge}
                        </span>
                      )}
                    </button>
                  );
                })}
              </div>
            </div>
          </div>

          {/* Bottom session controls */}
          <div className="px-3 pt-4 mx-3 mb-1 border-t border-gray-100/80 space-y-0.5">
            <div className="px-1 pb-2 text-[9px] text-gray-400 font-extrabold tracking-widest uppercase">Session</div>
            <button
              onClick={() => {
                setPasswordStatus({ loading: false, error: '', success: '' });
                setPasswordForm({ currentPassword: '', newPassword: '', confirmPassword: '' });
                setIsPasswordModalOpen(true);
              }}
              className="w-full h-9 px-3.5 rounded-xl flex items-center gap-2.5 font-semibold text-xs text-gray-600 hover:bg-gray-100 transition-all"
            >
              <Key className="w-4 h-4 text-gray-400" />
              Change Password
            </button>
            <button
              onClick={handleLogout}
              className="w-full h-9 px-3.5 rounded-xl flex items-center gap-2.5 font-semibold text-xs text-rose-500 hover:bg-rose-50 transition-all"
            >
              <LogOut className="w-4 h-4" />
              Sign Out
            </button>
          </div>
        </aside>

        {/* Overlay backdrop for mobile sidebar */}
        {sidebarOpen && (
          <div 
            onClick={() => setSidebarOpen(false)} 
            className="fixed inset-0 z-40 bg-black/40 backdrop-blur-xs lg:hidden transition-opacity"
            aria-label="Close sidebar"
          ></div>
        )}

        {/* Main Content Area */}
        <main className="flex-1 overflow-y-auto p-3 sm:p-5 lg:p-8 naoja-admin-bg">
          {/* Mobile & Tablet Quick Tab Switcher Bar */}
          <div className="lg:hidden mb-4 overflow-x-auto no-scrollbar py-0.5 -mx-1 px-1">
            <div className="flex items-center gap-1.5 w-max">
              {[...biMenuItems, ...storeMenuItems].map((item) => {
                const Icon = item.icon;
                const isActive = activeTab === item.id;
                return (
                  <button
                    key={item.id}
                    onClick={() => setActiveTab(item.id)}
                    className={`h-9 px-3 rounded-xl text-xs font-bold flex items-center gap-1.5 shrink-0 transition-all cursor-pointer touch-manipulation ${
                      isActive
                        ? 'bg-brand-600 text-white shadow-sm'
                        : 'bg-white text-gray-700 border border-gray-200 hover:bg-gray-50'
                    }`}
                  >
                    <Icon className={`w-3.5 h-3.5 ${isActive ? 'text-white' : 'text-gray-500'}`} />
                    <span>{item.label}</span>
                    {item.badge && (
                      <span className={`text-[10px] px-1.5 py-0.2 rounded-full font-black ${isActive ? 'bg-white/25 text-white' : 'bg-brand-100 text-brand-700'}`}>
                        {item.badge}
                      </span>
                    )}
                  </button>
                );
              })}
            </div>
          </div>

          {loading ? (
            <div className="flex flex-col items-center justify-center py-24 text-gray-400 font-medium space-y-4">
              <div className="w-12 h-12 rounded-2xl flex items-center justify-center text-white font-display font-black text-2xl animate-pulse" style={{background: 'linear-gradient(135deg, #1D4ED8, #ff6b00)'}}>
                N
              </div>
              <span className="text-sm font-semibold text-gray-500">Loading Naoja Control Hub...</span>
            </div>
          ) : (
            <>
              {/* Tab 1: LIVE OVERVIEW / COMMAND CENTRE */}
              {activeTab === 'overview' && (
                <LiveOverviewTab
                  liveData={liveAnalytics}
                  events={liveEvents}
                  soundEnabled={soundEnabled}
                  setSoundEnabled={setSoundEnabled}
                />
              )}

              {/* Tab 2: SALES & REVENUE ANALYTICS */}
              {activeTab === 'sales_analytics' && (
                <SalesAnalyticsTab
                  summaryData={salesSummary}
                  categoryData={categoryAnalytics}
                  range={salesRange}
                  setRange={setSalesRange}
                  loading={analyticsLoading}
                />
              )}

              {/* Tab 3: PREDICTIVE FORECAST & TARGETS */}
              {activeTab === 'forecast' && (
                <ForecastPlanningTab
                  forecastData={forecastData}
                  insightsData={insightsData}
                  targetData={targetData}
                  onRefresh={fetchAllAnalytics}
                />
              )}

              {/* Tab 4: INVENTORY INTELLIGENCE */}
              {activeTab === 'inventory_intel' && (
                <InventoryIntelligenceTab
                  inventoryData={inventoryHealth}
                  loading={analyticsLoading}
                />
              )}

              {/* Tab 5: M-PESA & PAYMENT HEALTH */}
              {activeTab === 'payment_health' && (
                <PaymentHealthTab
                  paymentData={paymentData}
                  range={paymentRange}
                  setRange={setPaymentRange}
                />
              )}

              {/* Tab 6: ORDER FULFILLMENT & LOGISTICS */}
              {activeTab === 'fulfillment' && (
                <FulfillmentTab
                  fulfillmentData={fulfillmentData}
                  range={fulfillmentRange}
                  setRange={setFulfillmentRange}
                />
              )}

              {/* Tab 7: ALERTS CENTRE */}
              {activeTab === 'alerts' && (
                <AlertsCentreTab
                  alertsData={alertsData}
                  onAcknowledge={(id) => setAlertsData(prev => prev.map(a => a.id === id ? { ...a, acknowledged: true } : a))}
                  onRefresh={fetchAllAnalytics}
                />
              )}

              {/* Tab 2: PRODUCTS */}
              {activeTab === 'products' && (
                <div className="space-y-6">
                  <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
                    <div>
                      <h1 className="font-display text-2xl lg:text-3xl font-extrabold text-gray-950 tracking-tight">Product Catalogue</h1>
                      <p className="text-xs text-gray-500 mt-1">Manage catalog inventory, pricing, and stock availability.</p>
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
                  <div className="bg-white border border-gray-100 rounded-2xl p-4 sm:p-5 shadow-sm flex flex-col md:flex-row gap-4 items-center justify-between">
                    <div className="relative w-full md:w-80">
                      <Search className="absolute left-3.5 top-3 w-4 h-4 text-gray-400" />
                      <input
                        type="text"
                        placeholder="Search product name or slug..."
                        value={prodSearch}
                        onChange={(e) => setProdSearch(e.target.value)}
                        className="w-full h-10 pl-10 pr-4 border border-gray-200/80 rounded-xl text-xs focus:border-brand-500 focus:ring-2 focus:ring-brand-50 outline-none"
                      />
                    </div>
                    <div className="w-full md:w-auto flex gap-3.5 self-stretch md:self-auto">
                      <select
                        value={prodCategoryFilter}
                        onChange={(e) => setProdCategoryFilter(e.target.value)}
                        className="flex-1 md:w-48 h-10 border border-gray-200/80 rounded-xl px-3.5 text-xs focus:border-brand-500 outline-none bg-white font-semibold"
                      >
                        <option value="">All Categories</option>
                        {categories.map(cat => (
                          <option key={cat.id} value={cat.id}>{cat.name}</option>
                        ))}
                      </select>
                    </div>
                  </div>

                  {/* Products Table */}
                  <div className="bg-white border border-gray-100 rounded-2xl shadow-sm overflow-hidden">
                    <div className="overflow-x-auto">
                      <table className="w-full text-left border-collapse text-xs min-w-[640px]">
                        <thead>
                          <tr className="bg-slate-50/80 border-b border-gray-100 text-slate-500 uppercase tracking-wider font-extrabold text-[10px] h-11">
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
                            filteredProducts
                              .slice((prodPage - 1) * adminPageSize, prodPage * adminPageSize)
                              .map(prod => {
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

                    {/* Products Pagination */}
                    {filteredProducts.length > adminPageSize && (
                      <div className="px-6 py-3 border-t border-gray-100 bg-gray-50/50 flex items-center justify-between text-xs">
                        <span className="text-gray-500 font-medium">
                          Showing {(prodPage - 1) * adminPageSize + 1} to {Math.min(prodPage * adminPageSize, filteredProducts.length)} of {filteredProducts.length} products
                        </span>
                        <div className="flex items-center gap-2">
                          <button
                            onClick={() => setProdPage(p => Math.max(1, p - 1))}
                            disabled={prodPage === 1}
                            className="px-3 py-1 border border-gray-200 rounded-lg font-semibold text-gray-700 disabled:opacity-40 hover:bg-white transition-colors"
                          >
                            Previous
                          </button>
                          <span className="px-2 font-bold text-gray-700">
                            Page {prodPage} of {Math.ceil(filteredProducts.length / adminPageSize)}
                          </span>
                          <button
                            onClick={() => setProdPage(p => Math.min(Math.ceil(filteredProducts.length / adminPageSize), p + 1))}
                            disabled={prodPage === Math.ceil(filteredProducts.length / adminPageSize)}
                            className="px-3 py-1 border border-gray-200 rounded-lg font-semibold text-gray-700 disabled:opacity-40 hover:bg-white transition-colors"
                          >
                            Next
                          </button>
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* Tab 3: CATEGORIES */}
              {activeTab === 'categories' && (
                <div className="space-y-6">
                  <div className="flex justify-between items-center">
                    <div>
                      <h1 className="font-display text-2xl lg:text-3xl font-extrabold text-gray-950 tracking-tight">Category Catalog</h1>
                      <p className="text-xs text-gray-500 mt-1">Configure shop categories, icons, and product counts.</p>
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
                      <div key={cat.id} className="bg-white border border-gray-100 rounded-2xl p-5 shadow-sm hover:shadow-md hover:border-brand-200 transition-all flex items-center justify-between">
                        <div className="flex items-center gap-4">
                          <span className="text-2xl p-3 bg-slate-50 border border-gray-100 rounded-xl">{cat.icon || '📁'}</span>
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
                    <h1 className="font-display text-2xl lg:text-3xl font-extrabold text-gray-950 tracking-tight">Order Management & Fulfillment</h1>
                    <p className="text-xs text-gray-500 mt-1">Track customer checkouts, M-Pesa receipts, and delivery dispatch statuses.</p>
                  </div>

                  {/* Search and filter toolbar */}
                  <div className="bg-white border border-gray-100 rounded-2xl p-4 sm:p-5 shadow-sm flex flex-col md:flex-row gap-4 items-center justify-between">
                    <div className="relative w-full md:w-80">
                      <Search className="absolute left-3.5 top-3 w-4 h-4 text-gray-400" />
                      <input
                        type="text"
                        placeholder="Search ref or customer name..."
                        value={orderSearch}
                        onChange={(e) => setOrderSearch(e.target.value)}
                        className="w-full h-10 pl-10 pr-4 border border-gray-200/80 rounded-xl text-xs focus:border-brand-500 focus:ring-2 focus:ring-brand-50 outline-none"
                      />
                    </div>
                    <div className="w-full md:w-auto flex flex-wrap md:flex-nowrap gap-3 self-stretch md:self-auto">
                      <select
                        value={orderStatusFilter}
                        onChange={(e) => setOrderStatusFilter(e.target.value)}
                        className="flex-1 md:w-36 h-10 border border-gray-200/80 rounded-xl px-3 text-xs focus:border-brand-500 outline-none bg-white font-semibold"
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
                        className="flex-1 md:w-40 h-10 border border-gray-200/80 rounded-xl px-3 text-xs focus:border-brand-500 outline-none bg-white font-semibold"
                      >
                        <option value="">All Payments</option>
                        <option value="pending">Payment Pending</option>
                        <option value="paid">Payment Paid</option>
                        <option value="failed">Payment Failed</option>
                      </select>
                    </div>
                  </div>

                  {/* Orders Table */}
                  <div className="bg-white border border-gray-100 rounded-2xl shadow-sm overflow-hidden">
                    <div className="overflow-x-auto">
                      <table className="w-full text-left border-collapse text-xs min-w-[700px]">
                        <thead>
                          <tr className="bg-slate-50/80 border-b border-gray-100 text-slate-500 uppercase tracking-wider font-extrabold text-[10px] h-11">
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
                            filteredOrders
                              .slice((orderPage - 1) * adminPageSize, orderPage * adminPageSize)
                              .map(order => (
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

                    {/* Orders Pagination */}
                    {filteredOrders.length > adminPageSize && (
                      <div className="px-6 py-3 border-t border-gray-100 bg-gray-50/50 flex items-center justify-between text-xs">
                        <span className="text-gray-500 font-medium">
                          Showing {(orderPage - 1) * adminPageSize + 1} to {Math.min(orderPage * adminPageSize, filteredOrders.length)} of {filteredOrders.length} orders
                        </span>
                        <div className="flex items-center gap-2">
                          <button
                            onClick={() => setOrderPage(p => Math.max(1, p - 1))}
                            disabled={orderPage === 1}
                            className="px-3 py-1 border border-gray-200 rounded-lg font-semibold text-gray-700 disabled:opacity-40 hover:bg-white transition-colors"
                          >
                            Previous
                          </button>
                          <span className="px-2 font-bold text-gray-700">
                            Page {orderPage} of {Math.ceil(filteredOrders.length / adminPageSize)}
                          </span>
                          <button
                            onClick={() => setOrderPage(p => Math.min(Math.ceil(filteredOrders.length / adminPageSize), p + 1))}
                            disabled={orderPage === Math.ceil(filteredOrders.length / adminPageSize)}
                            className="px-3 py-1 border border-gray-200 rounded-lg font-semibold text-gray-700 disabled:opacity-40 hover:bg-white transition-colors"
                          >
                            Next
                          </button>
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* Tab 5: REVIEWS */}
              {activeTab === 'reviews' && (
                <div className="space-y-6">
                  <div>
                    <h1 className="font-display text-2xl lg:text-3xl font-extrabold text-gray-950 tracking-tight">Reviews & Testimonials</h1>
                    <p className="text-xs text-gray-500 mt-1">Moderate customer ratings and verified purchase feedback.</p>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
                    {reviews.length === 0 ? (
                      <div className="bg-white border border-gray-100 rounded-2xl p-8 text-center text-xs text-gray-400 md:col-span-2">
                        No reviews in database.
                      </div>
                    ) : (
                      reviews.map(rev => {
                        const prod = products.find(p => p.id === rev.product_id);
                        return (
                          <div key={rev.id} className="bg-white border border-gray-100 rounded-2xl p-5 shadow-sm flex flex-col justify-between space-y-4">
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
                                    className="h-8 px-3.5 bg-green-600 hover:bg-green-700 text-white rounded-lg text-[10px] font-bold flex items-center gap-1 transition-colors"
                                  >
                                    <Check className="w-3.5 h-3.5" />
                                    Approve
                                  </button>
                                )}
                                <button
                                  onClick={() => handleRejectReview(rev.id)}
                                  className="h-8 px-3.5 border border-red-200 hover:bg-red-50 text-red-600 rounded-lg text-[10px] font-bold flex items-center gap-1 transition-colors"
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
                    <h1 className="font-display text-2xl lg:text-3xl font-extrabold text-gray-950 tracking-tight">Customer Inquiries</h1>
                    <p className="text-xs text-gray-500 mt-1">Review contact inquiries left by store visitors.</p>
                  </div>

                  <div className="space-y-4">
                    {messages.length === 0 ? (
                      <div className="bg-white border border-gray-100 rounded-2xl p-8 text-center text-xs text-gray-400">
                        No customer messages found.
                      </div>
                    ) : (
                      messages.map(msg => (
                        <div key={msg.id} className={`bg-white border rounded-2xl p-5 shadow-sm space-y-3 ${
                          !msg.is_read ? 'border-brand-500 ring-1 ring-brand-500' : 'border-gray-100'
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
                                className={`h-8 px-3.5 rounded-lg text-[10px] font-bold transition-colors ${
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
                    <h1 className="font-display text-2xl lg:text-3xl font-extrabold text-gray-950 tracking-tight">Newsletter Subscribers</h1>
                    <p className="text-xs text-gray-500 mt-1">Naoja VIP subscribers for promotion announcements and updates.</p>
                  </div>

                  <div className="bg-white border border-gray-100 rounded-2xl shadow-sm overflow-hidden">
                    <div className="overflow-x-auto">
                      <table className="w-full text-left border-collapse text-xs min-w-[580px]">
                        <thead>
                          <tr className="bg-slate-50/80 border-b border-gray-100 text-slate-500 uppercase tracking-wider font-extrabold text-[10px] h-11">
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
        <div className="fixed inset-0 z-50 overflow-y-auto naoja-modal-overlay flex items-center justify-center p-4">
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
        <div className="fixed inset-0 z-50 overflow-y-auto naoja-modal-overlay flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl w-full max-w-lg overflow-hidden shadow-xl max-h-[95vh] flex flex-col">
            
            <div className="px-6 py-4 border-b border-gray-100 flex items-center justify-between sticky top-0 bg-white">
              <h3 className="font-display text-lg font-extrabold text-gray-950 tracking-tight">
                {editingProduct ? 'Edit Product Details' : 'Add New Product to Catalogue'}
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
                  <label className="block text-[10px] font-bold text-gray-500 uppercase mb-1.5">Product Image</label>
                  <div className="border-2 border-dashed border-gray-200 rounded-xl overflow-hidden">
                    {/* Preview Area */}
                    {productForm.image_preview ? (
                      <div className="relative group">
                        <img
                          src={productForm.image_preview}
                          alt="Product preview"
                          className="w-full h-40 object-cover"
                        />
                        <div className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
                          <label htmlFor="product-image-upload" className="cursor-pointer bg-white text-gray-800 text-xs font-bold px-3 py-1.5 rounded-lg flex items-center gap-1.5">
                            <Upload className="w-3.5 h-3.5" /> Change Image
                          </label>
                        </div>
                        {uploadingImage && (
                          <div className="absolute inset-0 bg-white/80 flex items-center justify-center">
                            <span className="text-xs font-semibold text-brand-600 animate-pulse">Uploading...</span>
                          </div>
                        )}
                      </div>
                    ) : (
                      <label htmlFor="product-image-upload" className="flex flex-col items-center justify-center h-32 cursor-pointer hover:bg-gray-50 transition-colors">
                        {uploadingImage ? (
                          <span className="text-xs font-semibold text-brand-600 animate-pulse">Uploading...</span>
                        ) : (
                          <>
                            <ImageIcon className="w-8 h-8 text-gray-300 mb-2" />
                            <span className="text-xs font-semibold text-gray-500">Click to upload image</span>
                            <span className="text-[10px] text-gray-400 mt-0.5">JPG, PNG, WEBP up to 10MB</span>
                          </>
                        )}
                      </label>
                    )}
                    <input
                      id="product-image-upload"
                      type="file"
                      accept="image/*"
                      className="hidden"
                      onChange={handleImageFileChange}
                    />
                  </div>
                  {/* Also allow pasting a URL manually */}
                  <input
                    type="url"
                    value={productForm.image_url}
                    onChange={(e) => setProductForm({ ...productForm, image_url: e.target.value, image_preview: e.target.value })}
                    placeholder="Or paste image URL…"
                    className="mt-2 w-full h-9 border border-gray-200 rounded-xl px-3 text-[11px] focus:border-brand-500 outline-none text-gray-500"
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
        <div className="fixed inset-0 z-50 overflow-y-auto naoja-modal-overlay flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl w-full max-w-sm overflow-hidden shadow-xl flex flex-col">
            
            <div className="px-6 py-4 border-b border-gray-100 flex items-center justify-between">
              <h3 className="font-display text-lg font-extrabold text-gray-950 tracking-tight">
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

      {/* Password Change Modal */}
      {isPasswordModalOpen && (
        <div className="fixed inset-0 z-50 naoja-modal-overlay flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-md w-full shadow-2xl overflow-hidden">
            <div className="px-6 py-4 border-b border-gray-100 flex items-center justify-between">
              <h3 className="font-bold text-gray-900 text-base flex items-center gap-2">
                <Key className="w-4.5 h-4.5 text-brand-600" />
                Change Admin Password
              </h3>
              <button 
                onClick={() => setIsPasswordModalOpen(false)} 
                className="p-1 hover:bg-gray-100 rounded-lg text-gray-500"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <form onSubmit={handlePasswordSubmit} className="p-6 space-y-4 text-xs">
              {passwordStatus.error && (
                <div className="p-3 bg-red-50 border border-red-200 text-red-700 rounded-xl text-xs flex items-center gap-2">
                  <AlertTriangle className="w-4 h-4 shrink-0" />
                  <span>{passwordStatus.error}</span>
                </div>
              )}

              {passwordStatus.success && (
                <div className="p-3 bg-emerald-50 border border-emerald-200 text-emerald-700 rounded-xl text-xs flex items-center gap-2">
                  <CheckCircle className="w-4 h-4 shrink-0" />
                  <span>{passwordStatus.success}</span>
                </div>
              )}

              <div>
                <label className="block text-[10px] font-bold text-gray-500 uppercase mb-1.5">Current Password</label>
                <input
                  type="password"
                  required
                  value={passwordForm.currentPassword}
                  onChange={(e) => setPasswordForm({ ...passwordForm, currentPassword: e.target.value })}
                  placeholder="Enter current password"
                  className="w-full h-11 border border-gray-200 rounded-xl px-3.5 text-xs focus:border-brand-500 outline-none"
                />
              </div>

              <div>
                <label className="block text-[10px] font-bold text-gray-500 uppercase mb-1.5">New Password (min 6 characters)</label>
                <input
                  type="password"
                  required
                  value={passwordForm.newPassword}
                  onChange={(e) => setPasswordForm({ ...passwordForm, newPassword: e.target.value })}
                  placeholder="Enter new strong password"
                  className="w-full h-11 border border-gray-200 rounded-xl px-3.5 text-xs focus:border-brand-500 outline-none"
                />
              </div>

              <div>
                <label className="block text-[10px] font-bold text-gray-500 uppercase mb-1.5">Confirm New Password</label>
                <input
                  type="password"
                  required
                  value={passwordForm.confirmPassword}
                  onChange={(e) => setPasswordForm({ ...passwordForm, confirmPassword: e.target.value })}
                  placeholder="Repeat new password"
                  className="w-full h-11 border border-gray-200 rounded-xl px-3.5 text-xs focus:border-brand-500 outline-none"
                />
              </div>

              <div className="border-t border-gray-100 pt-4 flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setIsPasswordModalOpen(false)}
                  className="h-11 px-5 border border-gray-200 hover:bg-gray-50 text-gray-700 rounded-xl font-bold transition-all"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={passwordStatus.loading}
                  className="h-11 px-5 bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white rounded-xl font-bold transition-all"
                >
                  {passwordStatus.loading ? 'Updating...' : 'Update Password'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

    </div>
  );
}
