import axios from 'axios';

const request = axios.create({
  baseURL: 'https://1.12.248.26/api/v1',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
});

request.interceptors.response.use(
  (response) => {
    return response.data;
  },
  (error) => {
    // Handle errors (e.g., show toast)
    console.error('API Error:', error);
    return Promise.reject(error);
  }
);

export default request;
