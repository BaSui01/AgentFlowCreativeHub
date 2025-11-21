import { defineConfig } from 'orval';

export default defineConfig({
  agentFlow: {
    input: {
      target: '../backend/api/docs/swagger.json',
    },
    output: {
      mode: 'tags-split',
      target: 'src/api/generated/endpoint.ts',
      schemas: 'src/api/generated/model',
      client: 'react-query',
      prettier: true,
      override: {
        mutator: {
          path: 'src/api/instance.ts',
          name: 'customInstance',
        },
      },
    },
  },
});
