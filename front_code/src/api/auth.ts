import request from '../utils/request';

export interface SendCodeRequest {
  email: string;
}

export interface VerifyCodeLoginRequest {
  email: string;
  code: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface VerifyCodeRequest {
  username: string;
  email: string;
  password: string;
  code: string;
}

export interface LoginResponse {
  message: string;
  data: {
    user: {
      user_id: string;
      email: string;
      username: string;
    };
    access_token: string;
    refresh_token: string;
  };
}

export const authApi = {
  sendCode: (data: SendCodeRequest) => {
    return request.post<any, { message: string }>('/send_code', data);
  },
  
  verifyCodeLogin: (data: VerifyCodeLoginRequest) => {
    return request.post<any, LoginResponse>('/verify_code_login', data);
  },

  login: (data: LoginRequest) => {
    return request.post<any, LoginResponse>('/login', data);
  },

  register: (data: VerifyCodeRequest) => {
    return request.post<any, LoginResponse>('/register', data);
  }
};
