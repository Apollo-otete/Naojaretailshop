import React from 'react';
import { Link, useLocation } from 'react-router-dom';
import { Home, Grid, Flame, ShoppingCart, User } from 'lucide-react';
import { useCart } from '../lib/cart';

export default function MobileBottomNav({ onOpenCategories }) {
  const location = useLocation();
  const { itemCount } = useCart();
  const currentPath = location.pathname;

  const navItems = [
    {
      id: 'home',
      label: 'Home',
      icon: Home,
      to: '/',
      isActive: currentPath === '/',
    },
    {
      id: 'categories',
      label: 'Categories',
      icon: Grid,
      to: '/category/mobile-phones',
      onClick: onOpenCategories,
      isActive: currentPath.startsWith('/category'),
    },
    {
      id: 'deals',
      label: 'SuperDeals',
      icon: Flame,
      to: '/?filter=deals',
      isActive: location.search.includes('deals'),
      isFlame: true,
    },
    {
      id: 'cart',
      label: 'Cart',
      icon: ShoppingCart,
      to: '/cart',
      isActive: currentPath === '/cart',
      badge: itemCount > 0 ? itemCount : null,
    },
    {
      id: 'account',
      label: 'Account',
      icon: User,
      to: '/account',
      isActive: currentPath === '/account',
    },
  ];

  return (
    <nav
      aria-label="Mobile Navigation"
      className="md:hidden fixed bottom-0 left-0 right-0 z-40 bg-white/95 backdrop-blur-lg border-t border-gray-200/90 shadow-[0_-4px_16px_rgba(0,0,0,0.06)]"
      style={{ paddingBottom: 'max(0.5rem, env(safe-area-inset-bottom))' }}
    >
      <div className="grid grid-cols-5 h-14 items-center px-1">
        {navItems.map((item) => {
          const Icon = item.icon;
          const active = item.isActive;

          const content = (
            <div className="flex flex-col items-center justify-center py-1 relative">
              <div className="relative">
                <Icon
                  className={`w-5 h-5 transition-transform duration-200 ${
                    active
                      ? item.isFlame
                        ? 'text-retail-red scale-110 fill-retail-red'
                        : 'text-brand-600 scale-110'
                      : 'text-gray-500'
                  }`}
                />
                {item.badge && (
                  <span className="absolute -top-1.5 -right-2.5 bg-retail-red text-white text-[10px] font-black w-4 h-4 rounded-full flex items-center justify-center shadow-xs">
                    {item.badge > 99 ? '99+' : item.badge}
                  </span>
                )}
              </div>
              <span
                className={`text-[10px] font-bold mt-1 tracking-tight truncate max-w-[64px] ${
                  active
                    ? item.isFlame
                      ? 'text-retail-red'
                      : 'text-brand-600'
                    : 'text-gray-500'
                }`}
              >
                {item.label}
              </span>
              {active && (
                <span className="absolute bottom-0 w-6 h-0.5 bg-brand-500 rounded-full"></span>
              )}
            </div>
          );

          if (item.onClick) {
            return (
              <button
                key={item.id}
                type="button"
                onClick={item.onClick}
                className="w-full flex items-center justify-center touch-manipulation focus:outline-none"
              >
                {content}
              </button>
            );
          }

          return (
            <Link
              key={item.id}
              to={item.to}
              className="w-full flex items-center justify-center touch-manipulation focus:outline-none"
            >
              {content}
            </Link>
          );
        })}
      </div>
    </nav>
  );
}
