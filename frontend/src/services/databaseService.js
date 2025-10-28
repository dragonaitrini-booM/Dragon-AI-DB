import { apiService } from './apiService';

class DatabaseService {
  async listTables(database) {
    const response = await apiService.get(`/api/tables/${database}`);
    return response.tables;
  }

  async getTableSchema(tableName) {
    const response = await apiService.get(`/api/schema/public/${tableName}`);
    return response;
  }
}

export const databaseService = new DatabaseService();
