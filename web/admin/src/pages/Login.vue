<template>
  <div class="login-page">
    <form class="panel" @submit.prevent="handleSubmit">
      <h2>Sign in</h2>
      <label>
        Email
        <input v-model="form.email" type="email" placeholder="admin@example.com" required />
      </label>
      <label>
        Password
        <input v-model="form.password" type="password" required />
      </label>
      <button type="submit">Continue</button>
    </form>
  </div>
</template>

<script setup lang="ts">
import { reactive } from 'vue';
import { useRouter } from 'vue-router';
import { api } from '../services/api';

const router = useRouter();

const form = reactive({
  email: '',
  password: ''
});

const handleSubmit = async () => {
  try {
    await api.post('/api/auth/login', form);
    router.push('/');
  } catch (error) {
    console.error('Login failed', error);
  }
};
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #4338ca, #2563eb);
}

.panel {
  background: #fff;
  padding: 2.5rem 3rem;
  border-radius: 16px;
  width: 360px;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  box-shadow: 0 16px 48px rgba(30, 64, 175, 0.25);
}

label {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  font-weight: 600;
  color: #1f2937;
}

input {
  padding: 0.75rem 1rem;
  border-radius: 8px;
  border: 1px solid #cbd5f5;
  font-size: 1rem;
}

button {
  margin-top: 1rem;
  padding: 0.75rem 1rem;
  border-radius: 8px;
  border: none;
  background: #2563eb;
  color: #fff;
  font-weight: 600;
  cursor: pointer;
}
</style>

