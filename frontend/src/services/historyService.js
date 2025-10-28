import { apiService } from './apiService';

class HistoryService {
  async getHistory() {
    const response = await apiService.get('/api/history');
    return response.history;
  }

  async saveHistory(command, results) {
    return apiService.post('/api/history', { command, results });
  }
}

export const historyService = new HistoryService();
