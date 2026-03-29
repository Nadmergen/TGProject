import { get, derived } from 'svelte/store';
import { language } from './stores';

const dict = {
  ru: {
    myProfile: 'Мой профиль',
    contacts: 'Контакты',
    settings: 'Настройки',
    logout: 'Выход',
    language: 'Язык',
    setEmojiStatus: 'Установить эмодзи-статус',
    name: 'Имя',
    status: 'Статус',
    phone: 'Номер телефона',
    email: 'Email',
    save: 'Сохранить изменения',
    addContact: 'Добавить контакт',
    contactNamePrompt: 'Имя контакта',
    contactPhonePrompt: 'Телефон контакта (можно пусто)',
    savedOk: '✓ Изменения сохранены',
    savedErr: '✗ Ошибка сохранения'
  },
  en: {
    myProfile: 'My profile',
    contacts: 'Contacts',
    settings: 'Settings',
    logout: 'Logout',
    language: 'Language',
    setEmojiStatus: 'Set emoji status',
    name: 'Name',
    status: 'Status',
    phone: 'Phone',
    email: 'Email',
    save: 'Save changes',
    addContact: 'Add contact',
    contactNamePrompt: 'Contact name',
    contactPhonePrompt: 'Contact phone (optional)',
    savedOk: '✓ Saved',
    savedErr: '✗ Save failed'
  }
};

export function t(key) {
  const lang = get(language) || 'ru';
  return dict[lang]?.[key] ?? dict.ru[key] ?? key;
}

export function setLang(lang) {
  language.set(lang);
  try { localStorage.setItem('lang', lang); } catch (_) {}
}

// Реактивный словарь для использования через $currentDict в компонентах
export const currentDict = derived(language, (lang) => dict[lang] || dict.ru);

