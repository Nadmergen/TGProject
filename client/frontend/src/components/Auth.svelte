<script>
  import { currentUser, isLoggedIn } from '../stores';

  // Состояния: 'login' (вход) | 'register-email' (почта) | 'register-verify' (код)
  let step = 'login';

  let user = "", pass = "", email = "", code = "";
  let err = "", successMsg = "";
  let isLoading = false;

  async function handleAction() {
    err = "";
    successMsg = "";
    isLoading = true;

    try {
      // === ШАГ 1: ЛОГИН ===
      if (step === 'login') {
        if (!user.trim() || !pass.trim()) { err = "Заполните поля"; isLoading = false; return; }

        const res = await fetch('http://localhost:8080/api/auth/login', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ username: user.trim(), password: pass })
        });
        const data = await res.json();

        if (data.status === "success") {
          localStorage.setItem('token', data.token);
          currentUser.set({ username: user.trim(), id: data.user_id });
          isLoggedIn.set(true);
        } else {
          err = data.message || "Неверный логин или пароль";
        }
      }

      // === ШАГ 2: ЗАПРОС КОДА НА ПОЧТУ ===
      else if (step === 'register-email') {
        if (!email.trim()) { err = "Введите email"; isLoading = false; return; }

        const res = await fetch('http://localhost:8080/api/auth/init-register', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ email: email.trim() })
        });
        const data = await res.json();

        if (data.status === "success") {
          successMsg = "✅ Код отправлен на почту! Проверьте папку Спам.";
          step = 'register-verify'; // Переключаем на форму ввода кода
        } else {
          err = data.message || "Ошибка отправки кода";
        }
      }

      // === ШАГ 3: ПОДТВЕРЖДЕНИЕ КОДА И РЕГИСТРАЦИЯ ===
      else if (step === 'register-verify') {
        if (!code.trim() || !user.trim() || !pass.trim()) {
          err = "Заполните все поля"; isLoading = false; return;
        }

        // Бэкенд должен принять почту, код, придуманный логин и пароль
        const res = await fetch('http://localhost:8080/api/auth/register', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            email: email.trim(),
            code: code.trim(),
            username: user.trim(),
            password: pass
          })
        });
        const data = await res.json();

        if (data.status === "success") {
          successMsg = "✅ Аккаунт успешно создан!";
          localStorage.setItem('token', data.token);

          setTimeout(() => {
            currentUser.set({ username: user.trim(), id: data.user_id });
            isLoggedIn.set(true);
          }, 1500);
        } else {
          err = data.message || "Неверный код или логин уже занят";
        }
      }
    } catch (e) {
      err = "Ошибка соединения с сервером";
    }

    isLoading = false;
  }
</script>

<div class="auth-container">
  <div class="auth-box">
    <h2>
      {step === 'login' ? 'Вход в аккаунт' :
      (step === 'register-email' ? 'Регистрация' : 'Подтверждение Email')}
    </h2>

    {#if err}<div class="err-msg">{err}</div>{/if}
    {#if successMsg}<div class="success-msg">{successMsg}</div>{/if}

    <div class="form-group">
      {#if step === 'login'}
        <input type="text" bind:value={user} placeholder="Имя пользователя" disabled={isLoading} />
        <input type="password" bind:value={pass} placeholder="Пароль" disabled={isLoading} />

      {:else if step === 'register-email'}
        <input type="email" bind:value={email} placeholder="Ваш Email адрес" disabled={isLoading} />

      {:else if step === 'register-verify'}
        <p class="info-text">Мы отправили код на <b>{email}</b></p>
        <input type="text" bind:value={code} placeholder="Код из письма (например, 123456)" disabled={isLoading} />
        <input type="text" bind:value={user} placeholder="Придумайте логин" disabled={isLoading} />
        <input type="password" bind:value={pass} placeholder="Придумайте пароль" disabled={isLoading} />
      {/if}
    </div>

    <button class="main-btn" on:click={handleAction} disabled={isLoading}>
      {#if isLoading}
        Загрузка...
      {:else}
        {step === 'login' ? 'Войти' :
        (step === 'register-email' ? 'Получить код' : 'Завершить регистрацию')}
      {/if}
    </button>

    <div class="toggle-link">
      {#if step === 'login'}
        Нет аккаунта? <span on:click={() => { step = 'register-email'; err=''; successMsg=''; }}>Зарегистрироваться</span>
      {:else}
        Уже есть аккаунт? <span on:click={() => { step = 'login'; err=''; successMsg=''; }}>Войти</span>
      {/if}
    </div>
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
    width: 320px;
    text-align: center;
    box-shadow: 0 10px 25px rgba(0,0,0,0.3);
  }

  .logo {
    font-size: 50px;
    margin-bottom: 10px;
  }

  h2 {
    margin-bottom: 25px;
    font-weight: 500;
  }

  .input-group {
    margin-bottom: 15px;
  }

  input {
    width: 100%;
    padding: 12px;
    margin: 5px 0;
    background: #242f3d;
    border: 2px solid transparent;
    color: white;
    border-radius: 8px;
    box-sizing: border-box;
    outline: none;
    transition: 0.2s;
    font-size: 14px;
  }

  input:focus {
    border-color: #4faeef;
  }

  input:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .err-msg {
    color: #ef5350;
    font-size: 14px;
    margin-bottom: 15px;
    background: rgba(239, 83, 80, 0.1);
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
    cursor: not-allowed;
    opacity: 0.7;
  }

  .text-btn {
    background: none;
    border: none;
    color: #4faeef;
    cursor: pointer;
    margin-top: 15px;
    width: 100%;
    font-weight: 500;
    transition: 0.2s;
  }

  .text-btn:hover:not(:disabled) {
    color: #3b90d1;
  }

  .text-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .success-msg {
    color: #4caf50;
    font-size: 14px;
    margin-bottom: 15px;
    background: rgba(76, 175, 80, 0.1);
    padding: 8px;
    border-radius: 5px;
  }
</style>