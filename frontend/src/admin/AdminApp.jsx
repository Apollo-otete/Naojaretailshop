// Separate admin entry: keeps the admin console out of the storefront flow.
// Visit /admin (or /console, /backoffice) to sign in and use the role-aware portals.
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import AdminLogin from '../pages/AdminLogin';
import AdminDashboard from '../pages/AdminDashboard';

export default function AdminApp() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/admin" element={<AdminDashboard />} />
        <Route path="/admin/login" element={<AdminLogin />} />
        <Route path="/console" element={<AdminDashboard />} />
        <Route path="/console/login" element={<AdminLogin />} />
        <Route path="/backoffice" element={<AdminDashboard />} />
        <Route path="/backoffice/login" element={<AdminLogin />} />
        <Route path="*" element={<Navigate to="/admin/login" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
