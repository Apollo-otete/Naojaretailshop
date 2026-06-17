-- Create storage bucket for category images
INSERT INTO storage.buckets (id, name, public)
VALUES ('category-images', 'category-images', true)
ON CONFLICT (id) DO NOTHING;

-- Storage policy for category images
CREATE POLICY "category_images_public_read" ON storage.objects FOR SELECT TO public
  USING (bucket_id = 'category-images');

CREATE POLICY "category_images_authenticated_upload" ON storage.objects FOR INSERT TO authenticated
  WITH CHECK (bucket_id = 'category-images');

CREATE POLICY "category_images_authenticated_update" ON storage.objects FOR UPDATE TO authenticated
  USING (bucket_id = 'category-images');

CREATE POLICY "category_images_authenticated_delete" ON storage.objects FOR DELETE TO authenticated
  USING (bucket_id = 'category-images');
