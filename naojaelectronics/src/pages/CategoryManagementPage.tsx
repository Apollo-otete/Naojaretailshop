import { useState, useEffect, useRef } from 'react';
import { Link } from 'react-router-dom';
import { ArrowLeft, Upload, Trash2, Edit, Save, X, Image as ImageIcon } from 'lucide-react';
import Layout from '../components/Layout';
import { supabase } from '../lib/supabase';
import { Category } from '../types';

export default function CategoryManagementPage() {
  const [categories, setCategories] = useState<Category[]>([]);
  const [loading, setLoading] = useState(true);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editForm, setEditForm] = useState({ name: '', description: '', image_url: '' });
  const [uploading, setUploading] = useState(false);
  const [saving, setSaving] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    fetchCategories();
  }, []);

  async function fetchCategories() {
    setLoading(true);
    try {
      const { data, error } = await supabase
        .from('categories')
        .select('*')
        .order('name');

      if (error) throw error;
      if (data) setCategories(data);
    } catch (error) {
      console.error('Error fetching categories:', error);
    } finally {
      setLoading(false);
    }
  }

  function startEditing(category: Category) {
    setEditingId(category.id);
    setEditForm({
      name: category.name,
      description: category.description || '',
      image_url: category.image_url || '',
    });
  }

  function cancelEditing() {
    setEditingId(null);
    setEditForm({ name: '', description: '', image_url: '' });
  }

  async function handleImageUpload(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file) return;

    setUploading(true);
    try {
      const fileExt = file.name.split('.').pop();
      const fileName = `${Date.now()}.${fileExt}`;
      const filePath = `categories/${fileName}`;

      const { error: uploadError } = await supabase.storage
        .from('category-images')
        .upload(filePath, file);

      if (uploadError) throw uploadError;

      const { data } = supabase.storage.from('category-images').getPublicUrl(filePath);
      setEditForm({ ...editForm, image_url: data.publicUrl });
    } catch (error) {
      console.error('Error uploading image:', error);
      alert('Failed to upload image. Please try again.');
    } finally {
      setUploading(false);
    }
  }

  async function saveCategory() {
    if (!editingId) return;
    setSaving(true);
    try {
      const { error } = await supabase
        .from('categories')
        .update({
          name: editForm.name,
          description: editForm.description,
          image_url: editForm.image_url,
        })
        .eq('id', editingId);

      if (error) throw error;
      await fetchCategories();
      cancelEditing();
    } catch (error) {
      console.error('Error saving category:', error);
      alert('Failed to save category. Please try again.');
    } finally {
      setSaving(false);
    }
  }

  return (
    <Layout>
      <div className="max-w-7xl mx-auto px-4 py-8">
        {/* Breadcrumb */}
        <div className="flex items-center gap-4 mb-6">
          <Link to="/" className="flex items-center gap-2 text-gray-500 hover:text-gray-700">
            <ArrowLeft className="w-4 h-4" />
            Back to Home
          </Link>
        </div>

        <div className="flex items-center justify-between mb-6">
          <h1 className="font-serif text-3xl text-gray-900">Manage Categories</h1>
        </div>

        {loading ? (
          <div className="text-center py-12 text-gray-500">Loading...</div>
        ) : (
          <div className="bg-white border border-gray-200 rounded-card overflow-hidden">
            <div className="divide-y divide-gray-200">
              {categories.map((category) => (
                <div key={category.id} className="p-4">
                  {editingId === category.id ? (
                    <div className="space-y-4">
                      <div>
                        <label className="text-sm text-gray-600 block mb-1">Name</label>
                        <input
                          type="text"
                          value={editForm.name}
                          onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
                          className="w-full h-11 border border-gray-200 rounded-control px-3 text-sm"
                          placeholder="Category name"
                        />
                      </div>

                      <div>
                        <label className="text-sm text-gray-600 block mb-1">Description</label>
                        <textarea
                          value={editForm.description}
                          onChange={(e) => setEditForm({ ...editForm, description: e.target.value })}
                          className="w-full h-24 border border-gray-200 rounded-control px-3 py-2 text-sm resize-none"
                          placeholder="Category description"
                        />
                      </div>

                      <div>
                        <label className="text-sm text-gray-600 block mb-2">Image</label>
                        <div className="flex items-center gap-4">
                          {editForm.image_url ? (
                            <div className="relative">
                              <img
                                src={editForm.image_url}
                                alt="Category"
                                className="w-20 h-20 rounded-lg object-cover border border-gray-200"
                              />
                              <button
                                onClick={() => setEditForm({ ...editForm, image_url: '' })}
                                className="absolute -top-2 -right-2 w-6 h-6 bg-red-500 text-white rounded-full flex items-center justify-center"
                              >
                                <X className="w-4 h-4" />
                              </button>
                            </div>
                          ) : (
                            <button
                              onClick={() => fileInputRef.current?.click()}
                              disabled={uploading}
                              className="w-20 h-20 border-2 border-dashed border-gray-300 rounded-lg flex flex-col items-center justify-center text-gray-400 hover:border-gray-400 transition-colors"
                            >
                              {uploading ? (
                                <span className="text-xs">Uploading...</span>
                              ) : (
                                <>
                                  <Upload className="w-5 h-5 mb-1" />
                                  <span className="text-xs">Upload</span>
                                </>
                              )}
                            </button>
                          )}
                          <input
                            ref={fileInputRef}
                            type="file"
                            accept="image/*"
                            onChange={handleImageUpload}
                            className="hidden"
                          />
                        </div>
                      </div>

                      <div className="flex items-center gap-3">
                        <button
                          onClick={saveCategory}
                          disabled={saving || !editForm.name}
                          className="btn-primary flex items-center gap-2"
                        >
                          <Save className="w-4 h-4" />
                          {saving ? 'Saving...' : 'Save'}
                        </button>
                        <button
                          onClick={cancelEditing}
                          className="btn-secondary flex items-center gap-2"
                        >
                          <X className="w-4 h-4" />
                          Cancel
                        </button>
                      </div>
                    </div>
                  ) : (
                    <div className="flex items-center gap-4">
                      {category.image_url ? (
                        <img
                          src={category.image_url}
                          alt={category.name}
                          className="w-16 h-16 rounded-lg object-cover border border-gray-200"
                        />
                      ) : (
                        <div className="w-16 h-16 bg-surface rounded-lg flex items-center justify-center">
                          <ImageIcon className="w-6 h-6 text-gray-300" />
                        </div>
                      )}

                      <div className="flex-1 min-w-0">
                        <h3 className="font-semibold text-gray-900">{category.name}</h3>
                        {category.description && (
                          <p className="text-sm text-gray-500 truncate mt-0.5">{category.description}</p>
                        )}
                        <p className="text-xs text-gray-400 mt-1">
                          {category.product_count ?? 0} products
                        </p>
                      </div>

                      <button
                        onClick={() => startEditing(category)}
                        className="flex items-center gap-2 px-3 py-2 text-brand-500 hover:bg-brand-50 rounded-control transition-colors"
                      >
                        <Edit className="w-4 h-4" />
                        <span className="text-sm">Edit</span>
                      </button>
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </Layout>
  );
}
