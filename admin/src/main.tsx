import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { RouterProvider } from 'react-router-dom';
import { AdminProvider, ADMIN_TOKEN_KEY } from './app/admin-context';
import { router } from './app/router';
import { createAdminApi, createTokenStore } from './runtime';
import { Toaster } from './components/ui/sonner';
import './styles.css';

const savedToken = globalThis.localStorage?.getItem(ADMIN_TOKEN_KEY) ?? null;
const tokenStore = createTokenStore(savedToken);
const api = createAdminApi({ baseUrl: '', getToken: () => tokenStore.getState().token });

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AdminProvider api={api} tokenStore={tokenStore}>
      <RouterProvider router={router} />
      <Toaster />
    </AdminProvider>
  </StrictMode>,
);