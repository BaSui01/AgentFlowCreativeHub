# Frontend Architecture Spec: Automated API Integration

## Goal
Design and implement a frontend architecture that fully automates API integration using the backend's OpenAPI (Swagger) specification, eliminating manual API client code.

## Tech Stack
- **Core**: React 19 + TypeScript + Vite
- **State Management**: TanStack Query v5 (Server State)
- **UI Framework**: Ant Design (Matches existing dependency)
- **API Generator**: **Orval** (Replaces `openapi-typescript-codegen`)
  - *Reason*: Orval generates ready-to-use **React Query Hooks** (e.g., `useGetUsers()`), whereas the current tool only generates fetch functions, requiring manual wiring.
- **HTTP Client**: Axios

## Architecture Design

### 1. Automated Workflow
The "Source of Truth" is the backend code.
1. **Backend** generates `swagger.json` (Go/Swaggo).
2. **Frontend** runs `npm run gen:api`.
3. **Orval** reads `swagger.json` and generates:
   - TypeScript Interfaces (Models)
   - Axios Request Functions
   - **React Query Hooks** (`useQuery`, `useMutation` wrappers)

### 2. Directory Structure
```text
src/
├── api/
│   ├── generated/       # 🛑 GITIGNORED. Auto-generated hooks & types.
│   └── instance.ts      # Custom Axios instance (Interceptors for Auth/Logging).
├── components/          # Shared UI components.
├── features/            # Feature-based modules (e.g., Auth, Projects).
└── main.tsx             # App entry.
```

### 3. Implementation Plan
1.  **Install Dependencies**: Add `orval` and remove `openapi-typescript-codegen`.
2.  **Configure Axios**: Create `src/api/instance.ts` to handle base URLs and JWT tokens.
3.  **Configure Orval**: Create `orval.config.ts` mapping the backend `swagger.json` to the output directory.
4.  **Update Scripts**: Replace `generate:api` script.
5.  **Proof of Concept**: Generate the hooks and demonstrate usage in `App.tsx`.

## Verification
- Run the generator script.
- Confirm `src/api/generated` is populated with `.ts` files containing `useQuery` hooks.
- Verify the application compiles without type errors.
