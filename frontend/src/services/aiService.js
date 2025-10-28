import { apiService } from './apiService';

class AIService {
  async executeCommand(params) {
    return apiService.post('/api/execute', params);
  }
}

export const aiService = new AIService();
