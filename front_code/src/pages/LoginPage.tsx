import React, { useState, useEffect } from 'react';
import styles from './LoginPage.module.scss';
import ArknightsButton from '../components/ArknightsButton';
import ArknightsInput from '../components/ArknightsInput';
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

  const handleLogin = async () => {
    if (loginMethod === 'code' && (!email || !code)) {
      setMessage('请输入邮箱和验证码');
      return;
    }
    if (loginMethod === 'password' && (!email || !password)) {
      setMessage('请输入邮箱和密码');
      return;
    }
    
    try {
      setLoading(true);
      let response;
      if (loginMethod === 'code') {
        response = await authApi.verifyCodeLogin({ email, code });
      } else {
        response = await authApi.login({ email, password });
      }
      
      console.log('Login Success:', response);
      setMessage('登录成功。');
      
      localStorage.setItem('access_token', response.data.access_token);
      localStorage.setItem('user_info', JSON.stringify(response.data.user));
      localStorage.setItem('refresh_token', response.data.refresh_token);      
      // Delay redirect slightly to show success message
      setTimeout(() => {
        navigate('/dashboard');
      }, 1000);
    } catch (error) {
      setMessage('登录失败。访问拒绝。');
    } finally {
      setLoading(false);
    }
  };

  const toggleLoginMethod = () => {
    setLoginMethod(prev => prev === 'code' ? 'password' : 'code');
    setMessage('');
  };

  return (
    <div className={styles.container}>
      <div className={styles.loginBox}>
        <div className={styles.header}>
          <h1 className={styles.title}>
            {loginMethod === 'code' ? 'Chant_ELink' : 'Chant_PLink'}
          </h1>
        </div>
        <div 
          className={styles.backButton}
          style={{ 
            visibility: loginMethod === 'password' ? 'visible' : 'hidden',
            pointerEvents: loginMethod === 'password' ? 'auto' : 'none'
          }}
          onClick={() => {
            setLoginMethod('code');
            setMessage('');
          }}
        >
          {`< 返回`}
        </div>
        
        <div className={styles.formGroup}>
          <ArknightsInput 
            label="邮箱地址" 
            type="email" 
            placeholder="example@chant.com"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
          
          {loginMethod === 'code' ? (
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
          ) : (
            <ArknightsInput 
              label="密码" 
              type="password" 
              placeholder="********"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          )}
          
          <ArknightsButton 
            label={loading ? "CONNECTING..." : "CONNECT"} 
            onClick={handleLogin}
            disabled={loading}
            style={{ width: '100%', marginTop: '5px' }}
          />

          <div className={styles.bottomLinks}>
            <div 
              className={styles.linkItem}
              onClick={toggleLoginMethod}
            >
              {`> ${loginMethod === 'code' ? '密码登录' : '验证码登录'}`}
            </div>
            <Link to="/register" className={styles.linkItem}>
              {`> 注册账号`}
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

export default LoginPage;
