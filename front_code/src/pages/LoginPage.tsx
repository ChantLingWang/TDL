import React, { useState, useEffect } from 'react';
import styles from './LoginPage.module.scss';
import { authApi } from '../api/auth';
import { useNavigate, Link } from 'react-router-dom';

const LoginPage: React.FC = () => {
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [password, setPassword] = useState('');
  const [loginMethod, setLoginMethod] = useState<'code' | 'password'>('code');
  const [loading, setLoading] = useState(false);
  const [countdown, setCountdown] = useState(0);
  const [message, setMessage] = useState('');
  const navigate = useNavigate();

  useEffect(() => {
    let timer: ReturnType<typeof setInterval>;
    if (countdown > 0) {
      timer = setInterval(() => setCountdown((prev) => prev - 1), 1000);
    }
    return () => { if (timer) clearInterval(timer); };
  }, [countdown]);

  const handleSendCode = async () => {
    if (!email) { setMessage('请输入邮箱地址'); return; }
    try {
      setLoading(true);
      await authApi.sendCode({ email });
      setCountdown(60);
      setMessage('验证码发送成功');
    } catch {
      setMessage('验证码发送失败');
    } finally { setLoading(false); }
  };

  const handleLogin = async () => {
    if (loginMethod === 'code' && (!email || !code)) { setMessage('请输入邮箱和验证码'); return; }
    if (loginMethod === 'password' && (!email || !password)) { setMessage('请输入邮箱和密码'); return; }

    try {
      setLoading(true);
      const response = loginMethod === 'code'
        ? await authApi.verifyCodeLogin({ email, code })
        : await authApi.login({ email, password });

      setMessage('登录成功。');
      localStorage.setItem('access_token', response.data.access_token);
      localStorage.setItem('user_info', JSON.stringify(response.data.user));
      localStorage.setItem('refresh_token', response.data.refresh_token);
      setTimeout(() => navigate('/dashboard'), 800);
    } catch (error) {
      const msg = (error as any)?.response?.data?.detail || '登录失败';
      setMessage(msg);
    } finally { setLoading(false); }
  };

  return (
    <div className={styles.container}>
      <div className={styles.card}>
        <h1 className={styles.title}>Chant</h1>
        <p className={styles.subtitle}>登录你的账号</p>

        <div className={styles.field}>
          <label className={styles.label}>邮箱地址</label>
          <input
            className={styles.input}
            type='email'
            placeholder='example@chant.com'
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </div>

        {loginMethod === 'code' ? (
          <div className={styles.field}>
            <label className={styles.label}>验证码</label>
            <div className={styles.codeRow}>
              <input
                className={styles.input}
                placeholder='******'
                value={code}
                onChange={(e) => setCode(e.target.value)}
              />
              <button
                className={styles.secondaryBtn}
                onClick={handleSendCode}
                disabled={loading || countdown > 0}
              >
                {countdown > 0 ? countdown + 's' : '发送验证码'}
              </button>
            </div>
          </div>
        ) : (
          <div className={styles.field}>
            <label className={styles.label}>密码</label>
            <input
              className={styles.input}
              type='password'
              placeholder='********'
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
        )}

        <button className={styles.primaryBtn} onClick={handleLogin} disabled={loading}>
          {loading ? '登录中…' : '登录'}
        </button>

        <div className={styles.links}>
          <button className={styles.link} onClick={() => { setLoginMethod(loginMethod === 'code' ? 'password' : 'code'); setMessage(''); }}>
            {loginMethod === 'code' ? '密码登录' : '验证码登录'}
          </button>
          <Link className={styles.link} to='/register'>注册账号</Link>
        </div>

        {message && (
          <div className={[styles.msg, message.includes('成功') ? styles.msgOk : ''].join(' ')}>{message}</div>
        )}
      </div>
    </div>
  );
};

export default LoginPage;
