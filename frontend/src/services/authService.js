import { apiService } from './apiService';

class AuthService {
  async login(email, password) {
    const response = await apiService.post('/api/auth/login', { email, password });
    if (response.token) {
      apiService.setToken(response.token);
    }
    return response;
  }

  async register(email, password, name) {
    const response = await apiService.post('/api/auth/register', { email, password, name });
    if (response.token) {
      apiService.setToken(response.token);
    }
    return response;
  }

  async validateToken(token) {
    // This is a mock validation. The backend's auth middleware does the real validation.
    try {
        const payload = JSON.parse(atob(token.split('.')[1]));
        if (payload.exp * 1000 < Date.now()) {
            throw new Error("Token expired");
        }
        // This is not a secure way to get user data, but for this mock-up it will suffice.
        // A real app would have a /api/me endpoint.
        return { id: payload.user_id, email: payload.email, name: "User" };
    } catch (e) {
        throw new Error("Invalid token");
    }
  }
}

export const authService = new AuthService();
