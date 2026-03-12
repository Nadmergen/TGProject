<script>
  import { currentUser, isLoggedIn } from './stores';

  let step = 'login'; // login | register-email | verify-code | register-password | verify-2fa
  let email = '', code = '', username = '', password = '', phone = '';
  let err = '', successMsg = '';
  let isLoading = false;
  let userID = 0;

  async function stepRegisterEmail() {
      err = '';
      if (!email.trim()) { err = 'Email required'; return; }

      isLoading = true;
      try {
        const res = await fetch('http://localhost:8080/api/auth/init-register', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ email }) // Только email
        });
        const data = await res.json();

        if (data.status === 'success') {
          successMsg = '✅ Код отправлен на почту!';
          step = 'verify-code';
        } else {
          err = data.message;
        }
      } catch (e) {
        err = 'Ошибка соединения';
      }
      isLoading = false;
    }

  async function stepVerifyCode() {
    err = '';
    if (!code.trim()) { err = 'Code required'; return; }

    isLoading = true;
    try {
      const res = await fetch('http://localhost:8080/api/auth/verify-code', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, code, username, password, phone })
      });
      const data = await res.json();

      if (data.status === 'success') {
        localStorage.setItem('token', data.token);
        localStorage.setItem('user_id', data.user_id);
        currentUser.set({ username, id: data.user_id });
        isLoggedIn.set(true);
      } else {
        err = data.message;
      }
    } catch (e) {
      err = 'Connection error';
    }
    isLoading = false;
  }

  async function stepLogin() {
    err = '';
    if (!username.trim() || !password.trim()) { err = 'Fill all fields'; return; }

    isLoading = true;
    try {
      const res = await fetch('http://localhost:8080/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password })
      });
      const data = await res.json();

      if (data.requires_otp) {
        userID = data.user_id;
        step = 'verify-2fa';
        successMsg = '✅2FA code sent to email';
      } else if (data.status === 'success') {
        localStorage.setItem('token', data.token);
        localStorage.setItem('user_id', data.user_id);
        currentUser.set({ username, id: data.user_id });
        isLoggedIn.set(true);
      } else {
        err = data.message;
      }
    } catch (e) {
      err = 'Connection error';
    }
    isLoading = false;
  }

  async function stepVerify2FA() {
    err = '';
    if (!code.trim()) { err = '2FA code required'; return; }

    isLoading = true;
    try {
      const res = await fetch('http://localhost:8080/api/auth/verify-2fa', {
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
      } else {
        err = data.message;
      }
    } catch (e) {
      err = 'Connection error';
    }
    isLoading = false;
  }
</script>

<div class="auth-box">
  <div class="card">
    <div class="logo">💬</div>

    <!-- ВХОД -->
    {#if step === 'login'}
      <h2>Вход</h2>
      <div class="input-group">
        <input bind:value={username} placeholder="Логин" disabled={isLoading} />
        <input bind:value={password} type="password" placeholder="Пароль" disabled={isLoading} />
      </div>
      {#if err}<div class="err-msg">{err}</div>{/if}
      {#if successMsg}<div class="success-msg">{successMsg}</div>{/if}
      <button on:click={stepLogin} class="main-btn" disabled={isLoading}>
        {isLoading ? '⏳ Загрузка...' : 'Войти'}
      </button>
      <button class="text-btn" on:click={() => { step = 'register-email'; err = ''; }}>
        Создать аккаунт
      </button>

    <!-- РЕГИСТРАЦИЯ: EMAIL -->
    {:else if step === 'register-email'}
      <h2>Регистрация</h2>
      <div class="step-info">Шаг 1 из 3</div>
      <div class="input-group">
        <input bind:value={email} type="email" placeholder="Email" disabled={isLoading} />
        <input bind:value={phone} type="tel" placeholder="+7 (xxx) xxx-xx-xx" disabled={isLoading} />
      </div>
      {#if err}<div class="err-msg">{err}</div>{/if}
      {#if successMsg}<div class="success-msg">{successMsg}</div>{/if}
      <button on:click={stepRegisterEmail} class="main-btn" disabled={isLoading}>
        {isLoading ? '⏳ Отправка...' : 'Получить код'}
      </button>
      <button class="text-btn" on:click={() => { step = 'login'; err = ''; }}>
        Уже есть аккаунт? Войти
      </button>

    <!-- РЕГИСТРАЦИЯ: КОД ПОДТВЕРЖДЕНИЯ -->
    {:else if step === 'verify-code'}
      <h2>Подтверждение</h2>
      <div class="step-info">Шаг 2 из 3</div>
      <p class="info-text">Введите 6-значный код, отправленный на {email}</p>
      <div class="input-group">
        <input bind:value={code} placeholder="Код (000000)" maxlength="6" disabled={isLoading} />
        <input bind:value={username} placeholder="Логин" disabled={isLoading} />
        <input bind:value={password} type="password" placeholder="Пароль" disabled={isLoading} />
      </div>
      {#if err}<div class="err-msg">{err}</div>{/if}
      {#if successMsg}<div class="success-msg">{successMsg}</div>{/if}
      <button on:click={stepVerifyCode} class="main-btn" disabled={isLoading}>
        {isLoading ? '⏳ Проверка...' : 'Создать аккаунт'}
      </button>
      <button class="text-btn" on:click={() => { step = 'register-email'; code = ''; err = ''; }}>
        ← Назад
      </button>

    <!-- ВХОД: 2FA КОД -->
    {:else if step === 'verify-2fa'}
      <h2>Двухфакторная авторизация</h2>
      <p class="info-text">Введите код из письма</p>
      <div class="input-group">
        <input bind:value={code} placeholder="Код (000000)" maxlength="6" disabled={isLoading} />
      </div>
      {#if err}<div class="err-msg">{err}</div>{/if}
      {#if successMsg}<div class="success-msg">{successMsg}</div>{/if}
      <button on:click={stepVerify2FA} class="main-btn" disabled={isLoading}>
        {isLoading ? '⏳ Проверка...' : 'Подтвердить'}
      </button>
    {/if}
  </div>
</div>

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
</style>