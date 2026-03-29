<script>
    import { onMount } from 'svelte';
    import { currentUser, isLoggedIn, recipient, isSidebarOpen, innerWidth } from './stores';
    import { setActiveRecipient, initChat } from './chatService';
    import { API_URL } from './config';
    import Sidebar from './components/Sidebar.svelte';
    import Chat from './components/Chat.svelte';
    import Drawer from './components/Drawer.svelte';
    import CallOverlay from './components/CallOverlay.svelte';
    import ContactInfo from './components/ContactInfo.svelte';

    let step = 'login';
    let email = '', code = '', username = '', password = '';
    let err = '', successMsg = '';
    let isLoading = false;
    let userID = 0;
    let showInfo = false;

    // При изменении ширины окна
    $: if ($innerWidth <= 750 && $isSidebarOpen) isSidebarOpen.set(false);

    // Инициализация после логина
    $: if ($isLoggedIn) {
        initChat();
        if (!$recipient) {
            // "Избранное" (чат с самим собой) вместо несуществующего "общего чата"
            if ($currentUser?.id) setActiveRecipient($currentUser.id, 'Избранное');
        }
    }

    onMount(async () => {
        // Prevent auto-login "fall through" on stale token.
        const token = localStorage.getItem('token');
        if (!token) return;
        try {
            const res = await fetch(`${API_URL}/api/messages?page=1`, { headers: { 'Authorization': `Bearer ${token}` } });
            if (res.status === 401) {
                localStorage.removeItem('token');
                localStorage.removeItem('user_id');
                isLoggedIn.set(false);
                currentUser.set(null);
            }
        } catch (_) {}
    });

    async function stepRegisterEmail() {
        err = '';
        if (!email.trim()) { err = 'Email required'; return; }

        isLoading = true;
        try {
            const res = await fetch(`${API_URL}/api/auth/init-register`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ email })
            });
            const data = await res.json();
            if (data.status === 'success') {
                successMsg = '✅ Код отправлен на почту!';
                step = 'verify-code';
            } else { err = data.message; }
        } catch (e) { err = 'Ошибка соединения'; }
        isLoading = false;
    }

    async function stepVerifyCode() {
        err = '';
        if (!code.trim() || code.length !== 6) { err = 'Код должен содержать 6 цифр'; return; }
        if (!username.trim() || !password.trim()) { err = 'Введите логин и пароль'; return; }

        isLoading = true;
        try {
            const res = await fetch(`${API_URL}/api/auth/verify-code`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ email, code, username, password, phone: '' })
            });
            const data = await res.json();
            if (data.status === 'success') {
                localStorage.setItem('token', data.token);
                localStorage.setItem('user_id', data.user_id);
                currentUser.set({ username, id: data.user_id });
                isLoggedIn.set(true);
            } else { err = data.message; }
        } catch (e) { err = 'Connection error'; }
        isLoading = false;
    }

    async function stepLogin() {
        err = '';
        if (!username.trim() || !password.trim()) { err = 'Заполните все поля'; return; }

        isLoading = true;
        try {
            const res = await fetch(`${API_URL}/api/auth/login`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username, password })
            });
            const data = await res.json();
            if (data.requires_otp) {
                userID = data.user_id;
                step = 'verify-2fa';
                successMsg = '✅ Код 2FA отправлен';
            } else if (data.status === 'success') {
                localStorage.setItem('token', data.token);
                localStorage.setItem('user_id', data.user_id);
                currentUser.set({ username, id: data.user_id });
                isLoggedIn.set(true);
            } else { err = data.message; }
        } catch (e) { err = 'Connection error'; }
        isLoading = false;
    }

    async function stepVerify2FA() {
        err = '';
        if (!code.trim()) { err = 'Введите код'; return; }

        isLoading = true;
        try {
            const res = await fetch(`${API_URL}/api/auth/verify-2fa`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ user_id: userID, code })
            });
            const data = await res.json();
            if (data.status === 'success') {
                localStorage.setItem('token', data.token);
                localStorage.setItem('user_id', data.user_id);
                currentUser.set({ username, id: data.user_id });
                isLoggedIn.set(true);
            } else { err = data.message; }
        } catch (e) { err = 'Connection error'; }
        isLoading = false;
    }
</script>

<svelte:window bind:innerWidth={$innerWidth} />

{#if !$isLoggedIn}
    <div class="auth-box">
        <div class="card">
            <div class="logo">💬</div>
            {#if err}<div class="err-msg">{err}</div>{/if}
            {#if successMsg}<div class="success-msg">{successMsg}</div>{/if}

            {#if step === 'login'}
                <h2>Вход</h2>
                <div class="input-group">
                    <input bind:value={username} placeholder="Логин" disabled={isLoading} />
                    <input bind:value={password} type="password" placeholder="Пароль" disabled={isLoading} />
                </div>
                <button on:click={stepLogin} class="main-btn" disabled={isLoading}>
                    {isLoading ? '⏳ Загрузка...' : 'Войти'}
                </button>
                <button class="text-btn" on:click={() => { step = 'forgot-email'; err = ''; successMsg = ''; }}>
                    Забыли пароль?
                </button>
                <button class="text-btn" on:click={() => { step = 'qr-login'; err = ''; successMsg = ''; }}>
                    Войти по QR
                </button>
                <button class="text-btn" on:click={() => { step = 'register-email'; err = ''; }}>
                    Создать аккаунт
                </button>

            {:else if step === 'register-email'}
                <h2>Регистрация</h2>
                <div class="step-info">Шаг 1 из 2</div>
                <div class="input-group">
                    <input bind:value={email} type="email" placeholder="Email" disabled={isLoading} />
                </div>
                <button on:click={stepRegisterEmail} class="main-btn" disabled={isLoading}>
                    {isLoading ? '⏳ Отправка...' : 'Получить код'}
                </button>
                <button class="text-btn" on:click={() => { step = 'login'; err = ''; }}>
                    ← Назад к входу
                </button>

            {:else if step === 'verify-code'}
                <h2>Подтверждение</h2>
                <div class="step-info">Шаг 2 из 2</div>
                <p class="info-text">Введите код из письма {email}</p>
                <div class="input-group">
                    <input bind:value={code} placeholder="Код" maxlength="6" disabled={isLoading} />
                    <input bind:value={username} placeholder="Логин" disabled={isLoading} />
                    <input bind:value={password} type="password" placeholder="Пароль" disabled={isLoading} />
                </div>
                <button on:click={stepVerifyCode} class="main-btn" disabled={isLoading}>
                    Создать аккаунт
                </button>
                <button class="text-btn" on:click={() => { step = 'register-email'; err = ''; }}>
                    ← Назад
                </button>

            {:else if step === 'verify-2fa'}
                <h2>2FA</h2>
                <div class="input-group">
                    <input bind:value={code} placeholder="Код подтверждения" maxlength="6" disabled={isLoading} />
                </div>
                <button on:click={stepVerify2FA} class="main-btn" disabled={isLoading}>Подтвердить</button>
                <button class="text-btn" on:click={() => { step = 'login'; err = ''; }}>
                    ← Назад к входу
                </button>
            {:else if step === 'forgot-email'}
                <h2>Восстановление пароля</h2>
                <p class="info-text">Укажите email, на который зарегистрирован аккаунт.</p>
                <div class="input-group">
                    <input bind:value={email} type="email" placeholder="Ваш Email" disabled={isLoading} />
                </div>
                <button
                        class="main-btn"
                        disabled={isLoading}
                        on:click={async () => {
                            err = '';
                            successMsg = '';
                            if (!email.trim()) {
                                err = 'Введите email';
                                return;
                            }
                            isLoading = true;
                            try {
                                const res = await fetch(`${API_URL}/api/auth/forgot-password`, {
                                    method: 'POST',
                                    headers: { 'Content-Type': 'application/json' },
                                    body: JSON.stringify({ email })
                                });
                                const data = await res.json();
                                if (res.ok && data.status === 'success') {
                                    successMsg = 'Код для сброса отправлен на почту. Проверьте, пожалуйста, папку Спам.';
                                    step = 'forgot-verify';
                                } else {
                                    err = data.message || 'Не удалось отправить код';
                                }
                            } catch (_) {
                                err = 'Ошибка соединения';
                            }
                            isLoading = false;
                        }}
                >
                    {isLoading ? '⏳ Отправка...' : 'Получить код'}
                </button>
                <button class="text-btn" on:click={() => { step = 'login'; err = ''; successMsg = ''; }}>
                    ← Назад к входу
                </button>
            {:else if step === 'forgot-verify'}
                <h2>Новый пароль</h2>
                <p class="info-text">Введите код из письма и новый пароль для {email}.</p>
                <div class="input-group">
                    <input bind:value={code} placeholder="Код из письма" maxlength="6" disabled={isLoading} />
                    <input bind:value={password} type="password" placeholder="Новый пароль" disabled={isLoading} />
                </div>
                <button
                        class="main-btn"
                        disabled={isLoading}
                        on:click={async () => {
                            err = '';
                            successMsg = '';
                            if (!code.trim() || !password.trim()) {
                                err = 'Заполните все поля';
                                return;
                            }
                            isLoading = true;
                            try {
                                const res = await fetch(`${API_URL}/api/auth/reset-password`, {
                                    method: 'POST',
                                    headers: { 'Content-Type': 'application/json' },
                                    body: JSON.stringify({ email, code, new_password: password })
                                });
                                const data = await res.json();
                                if (res.ok && data.status === 'success') {
                                    successMsg = 'Пароль успешно обновлён. Теперь вы можете войти.';
                                    step = 'login';
                                } else {
                                    err = data.message || 'Не удалось обновить пароль';
                                }
                            } catch (_) {
                                err = 'Ошибка соединения';
                            }
                            isLoading = false;
                        }}
                >
                    {isLoading ? '⏳ Сохранение...' : 'Сохранить новый пароль'}
                </button>
                <button class="text-btn" on:click={() => { step = 'forgot-email'; err = ''; successMsg = ''; }}>
                    ← Назад
                </button>
            {:else if step === 'qr-login'}
                <h2>Вход по QR</h2>
                <p class="info-text">
                    Попросите приложение на другом устройстве с вашим аккаунтом показать QR и отсканируйте его.
                    После сканирования вы получите код, который нужно вставить ниже.
                </p>
                <div class="input-group">
                    <input bind:value={code} placeholder="QR код (из другого устройства)" disabled={isLoading} />
                </div>
                <button
                        class="main-btn"
                        disabled={isLoading}
                        on:click={async () => {
                            err = '';
                            successMsg = '';
                            if (!code.trim()) {
                                err = 'Введите код';
                                return;
                            }
                            isLoading = true;
                            try {
                                const res = await fetch(`${API_URL}/api/auth/qr-login`, {
                                    method: 'POST',
                                    headers: { 'Content-Type': 'application/json' },
                                    body: JSON.stringify({ token: code.trim() })
                                });
                                const data = await res.json();
                                if (res.ok && data.status === 'success') {
                                    localStorage.setItem('token', data.token);
                                    localStorage.setItem('user_id', data.user_id);
                                    currentUser.set({ username, id: data.user_id });
                                    isLoggedIn.set(true);
                                } else {
                                    err = data.message || 'Не удалось войти по QR';
                                }
                            } catch (_) {
                                err = 'Ошибка соединения';
                            }
                            isLoading = false;
                        }}
                >
                    {isLoading ? '⏳ Вход...' : 'Войти'}
                </button>
                <button class="text-btn" on:click={() => { step = 'login'; err = ''; successMsg = ''; }}>
                    ← Назад к входу
                </button>
            {/if}
        </div>
    </div>
{:else}
    <div class="app-layout">
        {#if $isSidebarOpen}
            <div class="sidebar-wrapper" class:mobile-overlay={$innerWidth <= 750}>
                <Sidebar />
            </div>
        {/if}
        <div class="chat-wrapper">
            <Chat toggleInfo={() => { showInfo = !showInfo; }} />
        </div>
        {#if showInfo && $innerWidth > 900}
            <div class="info-wrapper">
                <ContactInfo />
            </div>
        {/if}
    </div>
    <Drawer />
    <CallOverlay />
{/if}

<style>
  .auth-box {
    height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #0e1621;
  }

  .card {
    background: #17212b;
    padding: 30px;
    border-radius: 15px;
    width: 360px;
    text-align: center;
    box-shadow: 0 10px 40px rgba(0,0,0,0.5);
  }

  .logo {
    font-size: 50px;
    margin-bottom: 10px;
  }

  h2 {
    margin-bottom: 10px;
    font-weight: 500;
  }

  .step-info {
    font-size: 12px;
    color: #7f91a4;
    margin-bottom: 15px;
  }

  .info-text {
    font-size: 13px;
    color: #b0b9c1;
    margin-bottom: 15px;
  }

  .input-group {
    margin-bottom: 15px;
  }

  input {
    width: 100%;
    padding: 12px;
    margin: 6px 0;
    background: #242f3d;
    border: 1px solid #080e13;
    color: white;
    border-radius: 8px;
    box-sizing: border-box;
    outline: none;
    font-size: 14px;
    transition: 0.2s;
  }

  input:focus {
    border-color: #4faeef;
    box-shadow: 0 0 0 2px rgba(79, 174, 239, 0.1);
  }

  input:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .err-msg {
    color: #ef5350;
    font-size: 13px;
    margin-bottom: 15px;
    background: rgba(239, 83, 80, 0.1);
    padding: 8px;
    border-radius: 5px;
  }

  .success-msg {
    color: #4caf50;
    font-size: 13px;
    margin-bottom: 15px;
    background: rgba(76, 175, 80, 0.1);
    padding: 8px;
    border-radius: 5px;
  }

  .main-btn {
    width: 100%;
    padding: 12px;
    background: #4faeef;
    border: none;
    color: white;
    border-radius: 8px;
    cursor: pointer;
    font-weight: bold;
    font-size: 16px;
    transition: 0.2s;
  }

  .main-btn:hover:not(:disabled) {
    background: #3b90d1;
  }

  .main-btn:disabled {
    background: #2b5278;
    opacity: 0.7;
  }

  .text-btn {
    background: none;
    border: none;
    color: #4faeef;
    cursor: pointer;
    font-size: 13px;
    margin-top: 15px;
    width: 100%;
    transition: 0.2s;
  }

  .text-btn:hover {
    color: #3b90d1;
  }
  .app-layout {
      display: flex;
      height: 100vh;
      background: #0e1621;
  }
  .sidebar-wrapper {
      width: 300px;
      flex-shrink: 0;
      background: #17212b;
      border-right: 1px solid #080e13;
  }
  .sidebar-wrapper.mobile-overlay {
      position: absolute;
      left: 0;
      top: 0;
      height: 100vh;
      z-index: 10;
      box-shadow: 5px 0 15px rgba(0,0,0,0.5);
  }
  .chat-wrapper {
      flex: 1;
      min-width: 0;
  }

  .info-wrapper {
      width: 320px;
      flex-shrink: 0;
  }
</style>