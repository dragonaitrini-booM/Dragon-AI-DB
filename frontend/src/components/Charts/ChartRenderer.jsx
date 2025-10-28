import React, { useRef, useEffect } from 'react';
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  BarElement,
  LineElement,
  PointElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
} from 'chart.js';

ChartJS.register(
  CategoryScale,
  LinearScale,
  BarElement,
  LineElement,
  PointElement,
  ArcElement,
  Title,
  Tooltip,
  Legend
);

// Set global chart defaults for Trinidad theme
ChartJS.defaults.color = '#ffffff';
ChartJS.defaults.borderColor = '#333333';
ChartJS.defaults.backgroundColor = ['#000000', '#ffffff', '#e40000'];
ChartJS.defaults.font.family = "'Inter', 'system-ui', sans-serif";

const ChartRenderer = ({ spec, className = '' }) => {
  const canvasRef = useRef(null);
  const chartRef = useRef(null);

  useEffect(() => {
    if (!canvasRef.current || !spec) return;

    // Destroy existing chart
    if (chartRef.current) {
      chartRef.current.destroy();
    }

    const ctx = canvasRef.current.getContext('2d');

    // Apply performance optimizations
    const optimizedSpec = optimizeChartSpec(spec);

    try {
      chartRef.current = new ChartJS(ctx, optimizedSpec);
    } catch (error) {
      console.error('Chart rendering error:', error);
    }

    return () => {
      if (chartRef.current) {
        chartRef.current.destroy();
      }
    };
  }, [spec]);

  if (!spec) {
    return (
      <div className="chart-placeholder">
        <p>No chart data available</p>
      </div>
    );
  }

  return (
    <div className={`chart-container ${className}`}>
      <canvas
        ref={canvasRef}
        style={{
          maxWidth: '100%',
          maxHeight: '400px',
        }}
      />
    </div>
  );
};

// Chart.js performance optimization function
function optimizeChartSpec(spec) {
  const optimized = { ...spec };

  // Ensure options exist
  if (!optimized.options) {
    optimized.options = {};
  }

  // Add responsive behavior
  optimized.options.responsive = true;
  optimized.options.maintainAspectRatio = false;

  // Add animation settings
  optimized.options.animation = {
    duration: 750,
    easing: 'easeInOutQuart',
  };

  // Check dataset size for performance optimizations
  if (optimized.data?.datasets) {
    optimized.data.datasets.forEach((dataset) => {
      if (dataset.data?.length > 1000) {
        // Hide points for large datasets
        optimized.options.elements = {
          ...optimized.options.elements,
          point: { radius: 0 },
        };

        optimized.options.parsing = false;

        // Enable decimation for line charts with many points
        if (dataset.data.length > 5000) {
          optimized.options.decimation = {
            enabled: true,
            algorithm: 'lttb',
            samples: 1000,
          };
        }
      }
    });
  }

  // Optimize scales
  if (!optimized.options.scales) {
    optimized.options.scales = {};
  }

  // Add scale optimizations
  Object.keys(optimized.options.scales).forEach((scaleKey) => {
    const scale = optimized.options.scales[scaleKey];
    if (!scale.ticks) {
      scale.ticks = {};
    }

    // Limit number of ticks for performance
    scale.ticks.maxTicksLimit = 200;
    scale.ticks.source = 'auto';
  });

  return optimized;
}

export default ChartRenderer;
