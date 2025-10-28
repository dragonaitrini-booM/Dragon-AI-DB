import React, { useState, useCallback } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { toast } from 'react-hot-toast';
import { useDebounce } from '../../hooks/useDebounce';
import { aiService } from '../../services/aiService';
import { databaseService } from '../../services/databaseService';

import ChartRenderer from '../Charts/ChartRenderer';
import DataTable from '../DataTable/DataTable';
import ExportButtons from '../Export/ExportButtons';

import './QueryBuilder.css';

const QueryBuilder = () => {
  const [command, setCommand] = useState('');
  const [selectedTable, setSelectedTable] = useState('');
  const [context, setContext] = useState('');
  const [results, setResults] = useState(null);
  const [isPrivate, setIsPrivate] = useState(true);

  const debouncedCommand = useDebounce(command, 500);

  // Get available tables
  const { data: tables, isLoading: tablesLoading } = useQuery({
    queryKey: ['tables'],
    queryFn: () => databaseService.listTables('public'),
  });

  // Get table schema when selected
  const { data: tableSchema } = useQuery({
    queryKey: ['table-schema', selectedTable],
    queryFn: () => databaseService.getTableSchema(selectedTable),
    enabled: !!selectedTable,
  });

  // AI command execution
  const executeMutation = useMutation({
    mutationFn: (params) => aiService.executeCommand(params),
    onSuccess: (data) => {
      setResults(data);
      toast.success('Command executed successfully!');
    },
    onError: (error) => {
      console.error('Execution failed:', error);
      toast.error('Failed to execute command');
    },
  });

  // Auto-execute on command change (debounced)
  React.useEffect(() => {
    if (debouncedCommand.trim()) {
      handleExecute();
    }
  }, [debouncedCommand]);

  const handleExecute = useCallback(() => {
    if (!command.trim()) return;

    let pageContent = context;

    // If table is selected, add schema as context
    if (selectedTable && tableSchema) {
      const schemaContext = `
Table: ${tableSchema.name}
Columns: ${tableSchema.columns.map(col =>
  `${col.name} (${col.type}${col.nullable ? ', nullable' : ''})`
).join(', ')}
Row Count: ${tableSchema.row_count}
      `.trim();

      pageContent = pageContent ? `${pageContent}\n\n${schemaContext}` : schemaContext;
    }

    executeMutation.mutate({
      command,
      page_content: pageContent || null,
      is_private: isPrivate,
    });
  }, [command, context, selectedTable, tableSchema, isPrivate]);

  const handleTableSelect = (tableName) => {
    setSelectedTable(tableName);
    // Auto-generate context from selected table
    if (tableName) {
      const table = tables?.find(t => t.name === tableName);
      if (table) {
        setContext(`Analyze data from table: ${tableName}`);
      }
    }
  };

  return (
    <div className="query-builder">
      <div className="query-builder-header">
        <h2>AI Query Builder</h2>
        <p>Type natural language commands to analyze your data</p>
      </div>

      <div className="query-builder-content">
        {/* Left Panel - Input */}
        <div className="input-panel">
          <div className="input-section">
            <label className="input-label">
              Natural Language Command
            </label>
            <textarea
              value={command}
              onChange={(e) => setCommand(e.target.value)}
              placeholder="e.g., Show me sales trends by month, Create a bar chart of top products, Analyze customer demographics..."
              className="command-input"
              rows={4}
            />
          </div>

          <div className="input-section">
            <label className="input-label">
              Select Data Source
            </label>
            <select
              value={selectedTable}
              onChange={(e) => handleTableSelect(e.target.value)}
              className="table-select"
              disabled={tablesLoading}
            >
              <option value="">Choose a table...</option>
              {tables?.map((table) => (
                <option key={table.name} value={table.name}>
                  {table.name} ({table.row_count} rows)
                </option>
              ))}
            </select>
          </div>

          {selectedTable && tableSchema && (
            <div className="schema-info">
              <h4>Table Schema</h4>
              <div className="columns-list">
                {tableSchema.columns.map((column) => (
                  <div key={column.name} className="column-item">
                    <span className="column-name">{column.name}</span>
                    <span className="column-type">{column.type}</span>
                    {column.is_primary && (
                      <span className="column-badge primary">PK</span>
                    )}
                    {column.is_foreign && (
                      <span className="column-badge foreign">FK</span>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          <div className="input-section">
            <label className="input-label">
              Additional Context (Optional)
            </label>
            <textarea
              value={context}
              onChange={(e) => setContext(e.target.value)}
              placeholder="Provide additional context or specific requirements..."
              className="context-input"
              rows={3}
            />
          </div>

          <div className="options-section">
            <label className="checkbox-label">
              <input
                type="checkbox"
                checked={isPrivate}
                onChange={(e) => setIsPrivate(e.target.checked)}
              />
              <span>Private analysis (not shared)</span>
            </label>
          </div>

          <button
            onClick={handleExecute}
            disabled={!command.trim() || executeMutation.isLoading}
            className="execute-btn"
          >
            {executeMutation.isLoading ? 'Analyzing...' : 'Analyze Data'}
          </button>
        </div>

        {/* Right Panel - Results */}
        <div className="results-panel">
          {executeMutation.isLoading && (
            <div className="loading-state">
              <div className="loading-spinner" />
              <p>AI is analyzing your data...</p>
            </div>
          )}

          {results && (
            <div className="results-content">
              <div className="results-header">
                <h3>Analysis Results</h3>
                <div className="results-meta">
                  <span>Generated in {results.duration_ms}ms</span>
                  <span>Tokens used: {results.tokens_used}</span>
                </div>
              </div>

              {/* Chart Visualization */}
              {results.chart_spec && (
                <div className="chart-section">
                  <h4>Visualization</h4>
                  <ChartRenderer spec={results.chart_spec} />
                </div>
              )}

              {/* Data Table */}
              {results.table_data && results.table_data.length > 0 && (
                <div className="table-section">
                  <h4>Data Table</h4>
                  <DataTable data={results.table_data} />
                </div>
              )}

              {/* AI Insights */}
              {results.insights && results.insights.length > 0 && (
                <div className="insights-section">
                  <h4>Key Insights</h4>
                  <ul className="insights-list">
                    {results.insights.map((insight, index) => (
                      <li key={index} className="insight-item">
                        {insight}
                      </li>
                    ))}
                  </ul>
                </div>
              )}

              {/* Raw Content */}
              {results.content && (
                <div className="content-section">
                  <h4>Generated Content</h4>
                  <div className="content-display">
                    <pre>{results.content}</pre>
                  </div>
                </div>
              )}

              {/* Export Options */}
              <ExportButtons
                data={{
                  title: `Analysis: ${command.substring(0, 50)}...`,
                  chart_spec: results.chart_spec,
                  table_data: results.table_data,
                  insights: results.insights,
                  content: results.content,
                }}
              />
            </div>
          )}

          {!results && !executeMutation.isLoading && (
            <div className="empty-state">
              <p>Enter a command to start analyzing your data</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default QueryBuilder;
