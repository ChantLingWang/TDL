import React, { useState, useEffect } from 'react';
import styles from './LoginPage.module.scss'; // 复用登录页样式
import { authApi } from '../api/auth';
import { useNavigate, Link } from 'react-router-dom';

const RegisterPage: React.FC = () => {
  const [email, setEmail] = useState('');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [repeatPassword, setRepeatPassword] = useState('');
  const [code, setCode] = useState('');
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

  const handleRegister = async () => {
    if (!email || !code || !username || !password || !repeatPassword) {
      setMessage('请填写所有字段');
      return;
    }
    if (password !== repeatPassword) {
      setMessage('两次输入的密码不一致');
      return;
    }
    try {
      setLoading(true);
      const response = await authApi.register({ email, code, username, password });
      setMessage('注册成功，欢迎加入。');
      localStorage.setItem('access_token', response.data.access_token);
      localStorage.setItem('user_info', JSON.stringify(response.data.user));
      if (response.data.refresh_token) {
        localStorage.setItem('refresh_token', response.data.refresh_token);
      }
      setTimeout(() => navigate('/dashboard'), 800);
    } catch (error) {
      const msg = (error as any)?.response?.data?.detail || '注册失败';
      setMessage(msg);
    } finally { setLoading(false); }
  };

  return (
    <div className={styles.container}>
      <div className={styles.card}>
        <h1 className={styles.title}>Chant</h1>
        <p className={styles.subtitle}>创建新账号</p>

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

        <div className={styles.field}>
          <label className={styles.label}>用户名</label>
          <input
            className={styles.input}
            type='text'
            placeholder='你的昵称'
            value={username}
            onChange={(e) => setUsername(e.target.value)}
          />
        </div>

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

        <div className={styles.field}>
          <label className={styles.label}>确认密码</label>
          <input
            className={styles.input}
            type='password'
            placeholder='********'
            value={repeatPassword}
            onChange={(e) => setRepeatPassword(e.target.value)}
          />
        </div>

        <button className={styles.primaryBtn} onClick={handleRegister} disabled={loading}>
          {loading ? '注册中…' : '注册'}
        </button>

        <div className={styles.links}>
          <Link className={styles.link} to='/login'>返回登录</Link>
        </div>

        {message && (
          <div className={[styles.msg, message.includes('成功') ? styles.msgOk : ''].join(' ')}>{message}</div>
        )}
      </div>
    </div>
  );
};

export default RegisterPage;
