import React, { useState, useEffect } from 'react';
import styles from './LoginPage.module.scss'; // Reuse login styles
import ArknightsButton from '../components/ArknightsButton';
import ArknightsInput from '../components/ArknightsInput';
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
      timer = setInterval(() => {
        setCountdown((prev) => prev - 1);
      }, 1000);
    }
    return () => {
      if (timer) clearInterval(timer);
    };
  }, [countdown]);

  const handleSendCode = async () => {
    if (!email) {
      setMessage('请输入邮箱地址');
      return;
    }
    try {
      setLoading(true);
      await authApi.sendCode({ email });
      setCountdown(60);
      setMessage('验证码发送成功');
    } catch (error) {
      setMessage('验证码发送失败');
    } finally {
      setLoading(false);
    }
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
      console.log('Register Success:', response);
      setMessage('注册成功,欢迎加入。');
      
      localStorage.setItem('access_token', response.data.access_token);
      localStorage.setItem('user_info', JSON.stringify(response.data.user));
      
      setTimeout(() => {
        navigate('/dashboard');
      }, 1000);
    } catch (error) {
      setMessage('注册失败。');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className={styles.container}>
      <div className={styles.loginBox}>
        <div className={styles.header}>
          <h1 className={styles.title}>Chant_Registration</h1>
        </div>
        <div 
          className={styles.backButton}
          onClick={() => navigate('/login')}
        >
          {`< 返回`}
        </div>
        
        <div className={styles.formGroup}>
          <ArknightsInput 
            label="邮箱地址" 
            type="email" 
            placeholder="doctor@rhodes.island"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
          
          <div className={styles.codeRow}>
            <ArknightsInput 
              label="验证码" 
              className={styles.codeInput}
              placeholder="******"
              value={code}
              onChange={(e) => setCode(e.target.value)}
            />
            <ArknightsButton 
              label={countdown > 0 ? `${countdown}s` : "SEND CODE"} 
              onClick={handleSendCode}
              disabled={loading || countdown > 0}
            />
          </div>

          <ArknightsInput 
            label="用户名" 
            type="text" 
            placeholder="Dr. Name"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
          />

          <ArknightsInput 
            label="密码" 
            type="password" 
            placeholder="********"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />

          <ArknightsInput 
            label="确认密码" 
            type="password" 
            placeholder="********"
            value={repeatPassword}
            onChange={(e) => setRepeatPassword(e.target.value)}
          />
          
          <ArknightsButton 
            label={loading ? "PROCESSING..." : "REGISTER"} 
            onClick={handleRegister}
            disabled={loading}
            style={{ width: '100%', marginTop: '5px' }}
          />

          <div style={{ textAlign: 'center', marginTop: '10px' }}>
            <Link to="/login" className={styles.linkItem}>
              {`> 返回登录`}
            </Link>
          </div>
          
          {message && (
            <div style={{ 
              color: message.includes('成功') ? 'var(--ak-accent)' : 'var(--ak-error)',
              fontFamily: 'var(--ak-font-mono)',
              fontSize: '12px',
              marginTop: '10px',
              textAlign: 'center'
            }}>
              {`> ${message}`}
            </div>
          )}
        </div>
        
        <div className={styles.footer}>
          <p style={{ margin: '2px 0' }}>SYSTEM VERSION 3.0.1</p>
          <p style={{ margin: '2px 0' }}>COPYRIGHT © RHODES ISLAND</p>
        </div>
      </div>
    </div>
  );
};

export default RegisterPage;
