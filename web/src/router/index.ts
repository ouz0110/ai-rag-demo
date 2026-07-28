import { createRouter, createWebHistory } from 'vue-router';
import AuthView from '../views/AuthView.vue';
import ChatLayout from '../views/ChatLayout.vue';
import ChatIndex from '../views/ChatIndex.vue';
import ChatView from '../views/ChatView.vue';
import KBView from '../views/KBView.vue';

const routes = [
  {
    path: '/auth',
    name: 'Auth',
    component: AuthView,
    meta: { requiresAuth: false },
  },
  {
    path: '/',
    component: ChatLayout,
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        redirect: '/chat',
      },
      {
        path: 'chat',
        name: 'ChatIndex',
        component: ChatIndex,
      },
      {
        path: 'chat/:sessionId',
        name: 'ChatView',
        component: ChatView,
        props: true,
      },
      {
        path: 'kb',
        name: 'KBView',
        component: KBView,
      },
      {
        path: 'billing',
        name: 'BillingView',
        component: () => import('../views/BillingView.vue'),
      },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/chat',
  },
];

export const router = createRouter({
  history: createWebHistory(),
  routes,
});

router.beforeEach((to, _from, next) => {
  const token = localStorage.getItem('auth_token');
  if (to.meta.requiresAuth && !token) {
    next('/auth');
  } else if (to.path === '/auth' && token) {
    next('/chat');
  } else {
    next();
  }
});
