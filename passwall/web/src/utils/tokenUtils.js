// Token管理工具函数

// 保存Token到localStorage
export const saveToken = (token) => {
  if (token && typeof token === 'string') {
    localStorage.setItem('token', token.trim());
    return true;
  }
  return false;
};

// 从localStorage获取Token
export const getToken = () => {
  return localStorage.getItem('token');
};

// 清除Token
export const clearToken = () => {
  localStorage.removeItem('token');
};

// 检查是否有Token
export const hasToken = () => {
  const token = getToken();
  return !!token;
};

// 验证Token格式是否有效（这里只是一个简单的示例，实际验证逻辑可能更复杂）
export const isValidTokenFormat = (token) => {
  return token && typeof token === 'string' && token.trim().length > 0;
}; 