import React from 'react';
import { Helmet } from 'react-helmet-async';

export default function SEO({
  title,
  description = 'Shop quality electronics, solar equipment, hardware and home essentials at Naoja Ventures in Kakamega. Secure checkout and reliable service.',
  image = '',
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
      {image ? <meta property="og:image" content={image} /> : null}
      <meta property="og:url" content={currentUrl} />
      <meta property="og:site_name" content="Naoja Ventures" />

      {/* Twitter Cards */}
      <meta name="twitter:card" content={image ? 'summary_large_image' : 'summary'} />
      <meta name="twitter:title" content={siteTitle} />
      <meta name="twitter:description" content={description} />
      {image ? <meta name="twitter:image" content={image} /> : null}

      {/* Canonical Link */}
      <link rel="canonical" href={currentUrl} />
    </Helmet>
  );
}
