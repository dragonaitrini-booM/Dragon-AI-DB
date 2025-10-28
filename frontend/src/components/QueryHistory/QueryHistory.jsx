import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { historyService } from '../../services/historyService';
import './QueryHistory.css';

const QueryHistory = ({ onSelectHistory }) => {
  const { data: history, isLoading, error } = useQuery({
    queryKey: ['queryHistory'],
    queryFn: () => historyService.getHistory(),
  });

  if (isLoading) {
    return <div className="query-history-loading">Loading history...</div>;
  }

  if (error) {
    return <div className="query-history-error">Error loading history.</div>;
  }

  return (
    <div className="query-history">
      <h3 className="query-history-title">Query History</h3>
      <div className="query-history-list">
        {history?.map((item) => (
          <div
            key={item.id}
            className="query-history-item"
            onClick={() => onSelectHistory(item)}
          >
            <p className="query-history-command">{item.command}</p>
            <span className="query-history-date">
              {new Date(item.created_at).toLocaleString()}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
};

export default QueryHistory;
