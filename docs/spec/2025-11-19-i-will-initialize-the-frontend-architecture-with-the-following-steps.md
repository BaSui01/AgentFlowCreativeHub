I will initialize the Frontend Architecture with the following steps:

1.  **Install Core Dependencies**:
    -   `antd`: UI Component Library (as per requirements).
    -   `@ant-design/icons`: Icons for Ant Design.
    -   `react-router-dom`: For client-side routing.
    -   `@tanstack/react-query`: For efficient server state management.
    -   `axios`: HTTP client.

2.  **Setup Project Structure**:
    -   `src/app/providers`: Create `AppProviders.tsx` to wrap the app with `QueryClientProvider`, `ConfigProvider` (AntD theme), and `RouterProvider`.
    -   `src/app/router`: Create `AppRouter.tsx` to define routes.
    -   `src/shared/ui/layouts`: Create `MainLayout.tsx` (Sidebar + Header) and `AuthLayout.tsx`.
    -   `src/pages`: Create placeholder pages for `Dashboard` and `Login`.

3.  **Implementation Details**:
    -   Configure `vite.config.ts` for path aliases (e.g., `@/` -> `src/`) if not already set.
    -   Update `src/main.tsx` and `src/app/App.tsx` to integrate the new structure.

This establishes the "Standardization + Ecosystem Reuse" foundation required by the guidelines.