import { createRouter, createWebHistory } from 'vue-router';

const routes = [
  {
    path: '/login',
    name: 'login',
    component: () => import('../pages/Login.vue')
  },
  {
    path: '/',
    component: () => import('../layouts/BaseLayout.vue'),
    children: [
      {
        path: '',
        name: 'dashboard',
        component: () => import('../pages/Dashboard.vue')
      },
      {
        path: 'nodes',
        name: 'nodes',
        component: () => import('../pages/Nodes.vue')
      }
    ]
  }
];

const router = createRouter({
  history: createWebHistory(),
  routes
});

export default router;

