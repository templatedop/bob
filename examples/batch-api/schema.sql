-- Database schema for batch API example

CREATE TABLE IF NOT EXISTS categories (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    price DECIMAL(10, 2) NOT NULL CHECK (price > 0),
    stock INT NOT NULL DEFAULT 0 CHECK (stock >= 0),
    category_id INT NOT NULL REFERENCES categories(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_products_category ON products(category_id);
CREATE INDEX idx_products_stock ON products(stock);

-- Insert sample categories
INSERT INTO categories (name, description) VALUES
    ('Electronics', 'Electronic devices and gadgets'),
    ('Accessories', 'Computer and phone accessories'),
    ('Software', 'Software licenses and subscriptions'),
    ('Books', 'Technical books and documentation')
ON CONFLICT (name) DO NOTHING;

-- Insert sample products
INSERT INTO products (name, price, stock, category_id) VALUES
    ('Laptop Pro 15"', 1299.99, 25, 1),
    ('Wireless Mouse', 29.99, 150, 2),
    ('Mechanical Keyboard', 89.99, 75, 2),
    ('USB-C Hub', 49.99, 200, 2),
    ('Monitor 27"', 399.99, 40, 1)
ON CONFLICT DO NOTHING;
