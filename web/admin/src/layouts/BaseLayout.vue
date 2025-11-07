<template>
  <div class="layout">
    <aside class="sidebar">
      <h1 class="brand">VPN Master</h1>
      <nav>
        <RouterLink to="/">Dashboard</RouterLink>
        <RouterLink to="/nodes">Nodes</RouterLink>
      </nav>
    </aside>
    <main class="content">
      <NavBar />
      <section class="view">
        <RouterView />
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import NavBar from '../components/NavBar.vue';
</script>

<style scoped>
.layout {
  display: grid;
  grid-template-columns: 240px 1fr;
  min-height: 100vh;
}

.sidebar {
  background: #111827;
  color: #f9fafb;
  padding: 2rem 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 2rem;
}

.brand {
  font-size: 1.25rem;
  margin: 0;
}

nav {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

nav a {
  color: #d1d5db;
  font-weight: 500;
}

nav a.router-link-active {
  color: #60a5fa;
}

.content {
  background: #f5f7fa;
  display: flex;
  flex-direction: column;
}

.view {
  padding: 2rem;
}
</style>

