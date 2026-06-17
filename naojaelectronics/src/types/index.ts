export interface Product {
  id: string;
  name: string;
  slug: string;
  description: string;
  price: number;
  category_id: string;
  category_name: string;
  brand: string;
  image_url: string;
  in_stock: boolean;
  highlights: string[];
  specifications: Record<string, string>;
  created_at: string;
}

export interface Category {
  id: string;
  name: string;
  slug: string;
  description: string;
  image_url: string;
  product_count: number;
}

export interface CartItem {
  product: Product;
  quantity: number;
}

export interface User {
  id: string;
  email: string;
  full_name: string;
  phone: string;
}

export interface Order {
  id: string;
  user_id: string;
  status: 'pending' | 'confirmed' | 'processing' | 'delivered' | 'cancelled';
  total: number;
  fulfillment_type: 'delivery' | 'pickup';
  delivery_address?: DeliveryAddress;
  created_at: string;
  items: OrderItem[];
}

export interface DeliveryAddress {
  area: string;
  directions?: string;
}

export interface OrderItem {
  product_id: string;
  product_name: string;
  quantity: number;
  price: number;
}
