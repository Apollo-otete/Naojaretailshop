import React from 'react';
import { Helmet } from 'react-helmet-async';

export default function SEO({
  title,
  description = 'Shop quality solar equipment, electronics, hardware, kitchenware and general merchandise at Naoja Ventures in Lurambi, Kakamega. Fast delivery & M-Pesa payments.',
  image = 'https://images.unsplash.com/photo-1513094735237-8f2714d57c13?w=1200&h=630&fit=crop',
  url,
  type = 'website'
}) {
  const siteTitle = title 
    ? `${title} | Naoja Ventures Kakamega` 
    : 'Naoja Ventures — Quality Retail & Wholesale Store in Lurambi, Kakamega';
  
  const currentUrl = url || (typeof window !== 'undefined' ? window.location.href : 'https://naojaventures.com');

  return (
    <Helmet>
      {/* Standard Meta Tags */}
      <title>{siteTitle}</title>
      <meta name="description" content={description} />
      <meta name="keywords" content="Naoja Ventures, Kakamega shopping, Lurambi, solar panels, electronics Kenya, hardware Kenya, M-Pesa shopping" />

      {/* Open Graph / Facebook */}
      <meta property="og:type" content={type} />
      <meta property="og:title" content={siteTitle} />
      <meta property="og:description" content={description} />
      <meta property="og:image" content={image} />
      <meta property="og:url" content={currentUrl} />
      <meta property="og:site_name" content="Naoja Ventures" />

      {/* Twitter Cards */}
      <meta name="twitter:card" content="summary_large_image" />
      <meta name="twitter:title" content={siteTitle} />
      <meta name="twitter:description" content={description} />
      <meta name="twitter:image" content={image} />

      {/* Canonical Link */}
      <link rel="canonical" href={currentUrl} />
    </Helmet>
  );
}
