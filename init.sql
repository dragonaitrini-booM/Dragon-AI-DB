-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) DEFAULT 'user',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Sample data table
CREATE TABLE sales_data (
    id SERIAL PRIMARY KEY,
    date DATE NOT NULL,
    product VARCHAR(255) NOT NULL,
    revenue DECIMAL(10,2) NOT NULL,
    units_sold INTEGER NOT NULL,
    region VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert sample data
INSERT INTO sales_data (date, product, revenue, units_sold, region) VALUES
('2023-01-01', 'Product A', 1200.50, 24, 'North'),
('2023-01-02', 'Product B', 800.25, 16, 'South'),
('2023-01-03', 'Product A', 1500.75, 30, 'East'),
('2023-01-04', 'Product C', 950.00, 19, 'West');
