// Admin console entry (separate build from the storefront).
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import AdminLogin from '../pages/AdminLogin';
import AdminDashboard from '../pages/AdminDashboard';

export default function AdminApp() {
  return (
    <BrowserRouter basename="/admin">
      <Routes>
        <Route path="/login" element={<AdminLogin />} />
        <Route path="/" element={<AdminDashboard />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
