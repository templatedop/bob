-- Migration for batch testing
CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    stock INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    product_id INT NOT NULL REFERENCES products(id),
    quantity INT NOT NULL,
    total_price DECIMAL(10, 2) NOT NULL,
    order_date TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS inventory_log (
    id SERIAL PRIMARY KEY,
    product_id INT NOT NULL REFERENCES products(id),
    change_amount INT NOT NULL,
    reason TEXT,
    logged_at TIMESTAMP DEFAULT NOW()
);

-- Insert some test data
INSERT INTO products (name, price, stock) VALUES
    ('Widget A', 10.50, 100),
    ('Widget B', 25.00, 50),
    ('Widget C', 15.75, 75),
    ('Widget D', 30.00, 30),
    ('Widget E', 12.99, 200);
