<script>
    import { showDrawer, currentUser, isLoggedIn, language } from '../stores';
    import { fade, fly } from 'svelte/transition';
    import { API_URL } from '../config';
    import { t, setLang } from '../i18n';

    let activeTab = 'menu';
    let contacts = [];
    let isSaving = false;
    let saveStatus = '';

    let profileForm = {
        name: $currentUser?.username || '',
        status: 'Установить эмодзи-статус',
        phone: '',
        email: ''
    };

    function switchTab(tab) {
        activeTab = tab;
    }

    async function saveProfile() {
        isSaving = true;
        saveStatus = '';
        try {
            const token = localStorage.getItem('token');
            const res = await fetch(`${API_URL}/api/profile/update`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify({
                    username: profileForm.name,
                    status: profileForm.status,
                    phone: profileForm.phone,
                    email: profileForm.email
                })
            });
            const data = await res.json();
            if (!res.ok) throw new Error(data.error || 'Save failed');
            currentUser.set({ ...$currentUser, username: data.username, status: data.status });
            saveStatus = t('savedOk');
            setTimeout(() => saveStatus = '', 2000);
        } catch (err) {
            saveStatus = t('savedErr');
        }
        isSaving = false;
    }

    async function logout() {
        if (confirm("Вы уверены, что хотите выйти?")) {
            const token = localStorage.getItem('token');
            if (token) {
                try {
                    await fetch(`${API_URL}/api/auth/logout`, {
                        method: 'POST',
                        headers: { 'Authorization': `Bearer ${token}` }
                    });
                } catch (_) {
                    // ignore logout errors; we'll still clear local state
                }
            }
            localStorage.removeItem('token');
            localStorage.removeItem('user_id');
            isLoggedIn.set(false);
            showDrawer.set(false);
            currentUser.set(null);
        }
    }

    function addContact() {
        const name = prompt(t('contactNamePrompt'));
        if (!name) return;
        const phone = prompt(t('contactPhonePrompt')) || '';
        createContact(name, phone);
    }

    async function loadContacts() {
        try {
            const token = localStorage.getItem('token');
            const res = await fetch(`${API_URL}/api/contacts`, {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            const data = await res.json();
            if (res.ok) contacts = data;
        } catch (_) {}
    }

    async function createContact(name, phone) {
        try {
            const token = localStorage.getItem('token');
            const res = await fetch(`${API_URL}/api/contacts/add`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
                body: JSON.stringify({ name, phone })
            });
            await res.json();
            await loadContacts();
        } catch (_) {}
    }

    $: if (activeTab === 'contacts' && $showDrawer) loadContacts();
</script>

<svelte:window on:keydown={(e) => e.key === 'Escape' && $showDrawer && showDrawer.set(false)} />

{#if $showDrawer}
<div class="drawer-overlay" on:mousedown={() => showDrawer.set(false)} transition:fade={{duration: 150}}>
  <div class="drawer-content" on:mousedown|stopPropagation transition:fly={{x: -300, duration: 250}}>

    <!-- ГЛАВНОЕ МЕНЮ -->
    {#if activeTab === 'menu'}
      <div class="menu-view">
        <div class="drawer-header">
          <div class="big-avatar" style="background: #FF6B6B;">
            {$currentUser?.username ? $currentUser.username[0].toUpperCase() : 'U'}
          </div>
          <div class="user-meta">
            <div class="user-name">{$currentUser?.username}</div>
            <button class="status-btn" on:click={() => switchTab('profile')}>
              {$currentUser?.status || t('setEmojiStatus')}
              <span class="dropdown">∨</span>
            </button>
          </div>
        </div>

        <div class="menu-list">
          <button class="menu-btn" on:click={() => switchTab('profile')}>
            <span class="icon">👤</span>
            <span>{t('myProfile')}</span>
          </button>

          <button class="menu-btn" on:click={() => switchTab('contacts')}>
            <span class="icon">📞</span>
            <span>{t('contacts')}</span>
            <span class="badge">НОВОЕ</span>
          </button>

          <button class="menu-btn">
            <span class="icon">💬</span>
            <span>Создать группу</span>
          </button>

          <button class="menu-btn">
            <span class="icon">📢</span>
            <span>Создать канал</span>
          </button>

          <button class="menu-btn">
            <span class="icon">❤️</span>
            <span>Избранное</span>
          </button>

          <button class="menu-btn">
            <span class="icon">⚙️</span>
            <span>{t('settings')}</span>
          </button>

          <div class="lang-row">
            <span class="lang-label">{t('language')}</span>
            <select class="lang-select" bind:value={$language} on:change={(e) => setLang(e.target.value)}>
              <option value="ru">RU</option>
              <option value="en">EN</option>
            </select>
          </div>

          <button class="menu-btn toggle-btn">
            <span class="icon">🌙</span>
            <span>Ночной режим</span>
            <span class="toggle">
              <input type="checkbox" checked />
              <span class="slider"></span>
            </span>
          </button>

          <hr />

          <button class="menu-btn logout-btn" on:click={logout}>
            <span class="icon">🚪</span>
            <span>{t('logout')}</span>
          </button>
        </div>

        <div class="drawer-footer">
          <span class="app-info">Telegram Desktop</span>
          <span class="version">Версия 6.5.1 x64 — О программе</span>
        </div>
      </div>

    <!-- ПРОФИЛЬ -->
    {:else if activeTab === 'profile'}
      <div class="profile-view">
        <button type="button" class="back-btn" on:click={() => switchTab('menu')}>← Назад</button>

        <div class="profile-header">
          <div class="big-avatar" style="background: #FF6B6B;">
            {$currentUser?.username[0].toUpperCase()}
          </div>
          <h2>{t('myProfile')}</h2>
        </div>

        <div class="tab-content">
          <div class="profile-section">
            <label for="name-input">{t('name')}</label>
            <input
              id="name-input"
              type="text"
              bind:value={profileForm.name}
              placeholder="Ваше имя"
            />
          </div>

          <div class="profile-section">
            <label for="status-input">{t('status')}</label>
            <input
              id="status-input"
              type="text"
              bind:value={profileForm.status}
              placeholder="О чем вы думаете?"
            />
          </div>

          <div class="profile-section">
            <label for="phone-input">{t('phone')}</label>
            <input
              id="phone-input"
              type="tel"
              bind:value={profileForm.phone}
              placeholder="+7 (xxx) xxx-xx-xx"
            />
          </div>

          <div class="profile-section">
            <label for="email-input">{t('email')}</label>
            <input
              id="email-input"
              type="email"
              bind:value={profileForm.email}
              placeholder="your@email.com"
            />
          </div>

          <button
            class="save-btn"
            on:click={saveProfile}
            disabled={isSaving}
          >
            {#if isSaving}
              ⏳ Сохранение...
            {:else}
              💾 {t('save')}
            {/if}
          </button>

          {#if saveStatus}
            <div class="save-status {saveStatus.includes('✓') ? 'success' : 'error'}">
              {saveStatus}
            </div>
          {/if}
        </div>
      </div>

    <!-- КОНТАКТЫ -->
    {:else if activeTab === 'contacts'}
      <div class="contacts-view">
        <button type="button" class="back-btn" on:click={() => switchTab('menu')}>← Назад</button>

        <h2 class="contacts-title">{t('contacts')}</h2>

        <button class="add-contact-btn" on:click={addContact}>
          ➕ {t('addContact')}
        </button>

        <div class="contacts-list">
          {#if contacts.length > 0}
            {#each contacts as contact}
              <div class="contact-item">
                <div class="contact-avatar">{contact.name[0].toUpperCase()}</div>
                <div class="contact-info">
                  <div class="contact-name">{contact.name}</div>
                  <div class="contact-phone">{contact.phone}</div>
                </div>
                <button class="delete-contact-btn" on:click={() => { contacts = contacts.filter(c => c !== contact); }}>✕</button>
              </div>
            {/each}
          {:else}
            <div class="empty-contacts">Контактов пока нет</div>
          {/if}
        </div>
      </div>
    {/if}

  </div>
</div>
{/if}

<style>
  .drawer-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,0.6);
    z-index: 999;
    display: flex;
  }

  .drawer-content {
    width: 340px;
    height: 100%;
    background: #17212b;
    display: flex;
    flex-direction: column;
    overflow-y: auto;
  }

  .lang-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    padding: 10px 14px;
    border-radius: 12px;
    color: #b0b9c1;
    background: rgba(0,0,0,0.15);
    margin: 8px 10px;
  }
  .lang-label { font-size: 13px; }
  .lang-select {
    background: #242f3d;
    border: 1px solid #080e13;
    color: white;
    border-radius: 8px;
    padding: 6px 8px;
    outline: none;
  }

  /* ===== ГЛАВНОЕ МЕНЮ ===== */
  .menu-view {
    display: flex;
    flex-direction: column;
    height: 100%;
  }

  .drawer-header {
    padding: 20px;
    background: #242f3d;
    display: flex;
    gap: 15px;
    align-items: flex-start;
    flex-shrink: 0;
  }

  .big-avatar {
    width: 60px;
    height: 60px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 24px;
    font-weight: bold;
    flex-shrink: 0;
    color: white;
  }

  .user-meta {
    flex: 1;
  }

  .user-name {
    font-size: 18px;
    font-weight: bold;
    margin-bottom: 4px;
  }

  .status-btn {
    background: none;
    border: none;
    color: #7f91a4;
    font-size: 12px;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 6px;
    transition: 0.2s;
  }

  .status-btn:hover {
    color: #4faeef;
  }

  .dropdown {
    font-size: 14px;
  }

  .menu-list {
    flex: 1;
    padding: 10px 0;
    overflow-y: auto;
  }

  .menu-btn {
    width: 100%;
    background: none;
    border: none;
    color: #b0b9c1;
    padding: 12px 20px;
    text-align: left;
    font-size: 15px;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 16px;
    transition: 0.2s;
  }

  .menu-btn:hover {
    background: rgba(79, 174, 239, 0.1);
    color: white;
  }

  .menu-btn .icon {
    font-size: 20px;
    width: 24px;
    display: flex;
    justify-content: center;
  }

  .badge {
    margin-left: auto;
    background: #4faeef;
    color: white;
    font-size: 10px;
    font-weight: bold;
    padding: 2px 8px;
    border-radius: 4px;
  }

  .toggle-btn {
    position: relative;
  }

  .toggle {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .toggle input {
    display: none;
  }

  .slider {
    width: 40px;
    height: 22px;
    background: #0e1621;
    border-radius: 11px;
    position: relative;
    display: inline-block;
    border: 1px solid #080e13;
    transition: 0.3s;
  }

  .toggle input:checked + .slider {
    background: #4faeef;
  }

  .logout-btn {
    color: #7f91a4;
  }

  .logout-btn:hover {
    color: #b0b9c1;
    background: rgba(79, 174, 239, 0.05);
  }

  .drawer-footer {
    padding: 15px 20px;
    border-top: 1px solid #0e1621;
    flex-shrink: 0;
    font-size: 12px;
    color: #7f91a4;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .app-info {
    font-weight: 500;
  }

  .version {
    opacity: 0.7;
    cursor: pointer;
  }

  .version:hover {
    color: #4faeef;
  }

  hr {
    border: none;
    border-top: 1px solid #0e1621;
    margin: 8px 0;
  }

  /* ===== ПРОФИЛЬ ===== */
  .profile-view,
  .contacts-view {
    display: flex;
    flex-direction: column;
    height: 100%;
  }

  .back-btn {
    padding: 12px 20px;
    background: none;
    border: none;
    color: #4faeef;
    cursor: pointer;
    text-align: left;
    font-weight: 500;
    transition: 0.2s;
  }

  .back-btn:hover {
    color: #3b90d1;
  }

  .profile-header {
    padding: 20px;
    background: #242f3d;
    text-align: center;
    flex-shrink: 0;
  }

  .profile-header h2 {
    margin: 10px 0 0;
  }

  .tab-content {
    flex: 1;
    padding: 20px;
    overflow-y: auto;
  }

  .profile-section {
    margin-bottom: 15px;
  }

  .profile-section label {
    display: block;
    color: #7f91a4;
    font-size: 12px;
    font-weight: 600;
    margin-bottom: 6px;
    text-transform: uppercase;
  }

  .profile-section input {
    width: 100%;
    background: #0e1621;
    border: 1px solid #080e13;
    color: white;
    border-radius: 8px;
    padding: 10px;
    box-sizing: border-box;
    font-size: 14px;
    outline: none;
    transition: 0.2s;
  }

  .profile-section input:focus {
    border-color: #4faeef;
    box-shadow: 0 0 0 2px rgba(79, 174, 239, 0.1);
  }

  .save-btn {
    width: 100%;
    background: #4faeef;
    border: none;
    color: white;
    padding: 12px;
    border-radius: 8px;
    font-weight: bold;
    cursor: pointer;
    margin-top: 15px;
    transition: 0.2s;
  }

  .save-btn:hover:not(:disabled) {
    background: #3b90d1;
  }

  .save-btn:disabled {
    background: #2b5278;
    cursor: not-allowed;
    opacity: 0.7;
  }

  .save-status {
    margin-top: 10px;
    padding: 10px;
    border-radius: 6px;
    text-align: center;
    font-size: 13px;
    font-weight: 500;
  }

  .save-status.success {
    background: rgba(76, 175, 80, 0.15);
    color: #4caf50;
  }

  .save-status.error {
    background: rgba(229, 57, 53, 0.15);
    color: #e53935;
  }

  /* ===== КОНТАКТЫ ===== */
  .contacts-title {
    padding: 20px;
    margin: 0;
    background: #242f3d;
    flex-shrink: 0;
  }

  .add-contact-btn {
    margin: 15px 20px 0;
    width: calc(100% - 40px);
    background: #2b5278;
    border: none;
    color: white;
    padding: 12px;
    border-radius: 8px;
    cursor: pointer;
    font-weight: bold;
    transition: 0.2s;
  }

  .add-contact-btn:hover {
    background: #3d6a9a;
  }

  .contacts-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 15px 20px;
    overflow-y: auto;
  }

  .contact-item {
    display: flex;
    align-items: center;
    gap: 12px;
    background: #0e1621;
    padding: 12px;
    border-radius: 8px;
    border: 1px solid #080e13;
  }

  .contact-avatar {
    width: 40px;
    height: 40px;
    background: #2b5278;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: bold;
    flex-shrink: 0;
    color: white;
  }

  .contact-info {
    flex: 1;
  }

  .contact-name {
    font-weight: 500;
    color: white;
    font-size: 14px;
  }

  .contact-phone {
    font-size: 12px;
    color: #7f91a4;
  }

  .delete-contact-btn {
    background: none;
    border: none;
    color: #7f91a4;
    cursor: pointer;
    font-size: 16px;
    flex-shrink: 0;
    padding: 4px;
    transition: 0.2s;
  }

  .delete-contact-btn:hover {
    color: #b0b9c1;
    transform: scale(1.1);
  }

  .empty-contacts {
    text-align: center;
    color: #7f91a4;
    padding: 40px 20px;
    font-size: 14px;
  }
</style>