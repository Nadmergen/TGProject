import { writable, derived } from 'svelte/store';

export const currentUser = writable(null);          // { id, username }
export const recipient = writable(null);            // { id, username } активный собеседник
export const isLoggedIn = writable(false);
export const chatHistory = writable([]);

// UI состояния
export const showDrawer = writable(false);
export const isSidebarOpen = writable(true);
export const innerWidth = writable(window.innerWidth);

// Фильтры для медиа
export const mediaFiles = derived(chatHistory, ($h) => $h.filter(m => m.type === 'image'));
export const docFiles = derived(chatHistory, ($h) => $h.filter(m => m.type === 'file'));