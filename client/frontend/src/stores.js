import { writable, derived } from 'svelte/store';

// Пользователь
export const currentUser = writable(null);
export const isLoggedIn = writable(!!localStorage.getItem('token'));

// Активный чат
export const recipient = writable(null);

// История сообщений
export const chatHistory = writable([]);

// UI состояния
export const showDrawer = writable(false);
export const isSidebarOpen = writable(window.innerWidth > 750);
export const innerWidth = writable(window.innerWidth);
export const language = writable(localStorage.getItem('lang') || 'ru');

// Фильтры медиа
export const mediaFiles = derived(chatHistory, ($h) => $h.filter(m => m.type === 'image'));
export const docFiles = derived(chatHistory, ($h) => $h.filter(m => m.type === 'file'));
export const voiceMessages = derived(chatHistory, ($h) => $h.filter(m => m.type === 'voice'));

// Обновление innerWidth при ресайзе
if (typeof window !== 'undefined') {
    window.addEventListener('resize', () => {
        innerWidth.set(window.innerWidth);
        if (window.innerWidth <= 750) isSidebarOpen.set(false);
    });
}