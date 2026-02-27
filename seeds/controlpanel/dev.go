package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/argon2"
	"gopkg.in/yaml.v3"
)

const (
	devBrandID     = "brand_acme_dev_000000000001"
	devPartnerID   = "partner_acme_dev_000000000001"
	devPartner2ID  = "partner_bobs_dev_000000000001"
	devCustomerID  = "cust_acme_dev_000000000001"
	devCustomer2ID = "cust_bobs_dev_000000000001"
	devUserID      = "usr_acme_dev_000000000001"
	devUser2ID     = "usr_bobs_dev_000000000001"
)

type productsFile struct {
	Products []productEntry `yaml:"products"`
}

type productEntry struct {
	ID          string   `yaml:"id"`
	BrandID     string   `yaml:"brand_id"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Modules     []string `yaml:"modules"`
	Status      string   `yaml:"status"`
}

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	fmt.Println("Seeding control panel database...")

	// --- Static data ---

	fmt.Println("  Inserting partner...")
	_, err = pool.Exec(ctx,
		`INSERT INTO partners (id, brand_id, name, hostname, primary_color, status) VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (id) DO UPDATE SET hostname = EXCLUDED.hostname`,
		devPartnerID, devBrandID, "Acme Hosting", "home.massive-hosting.com", "264", "active")
	if err != nil {
		fmt.Fprintf(os.Stderr, "insert partner: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("  Inserting customer...")
	_, err = pool.Exec(ctx,
		`INSERT INTO customers (id, partner_id, name, email, status) VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`,
		devCustomerID, devPartnerID, "Acme Corp", "billing@acme-corp.test", "active")
	if err != nil {
		fmt.Fprintf(os.Stderr, "insert customer: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("  Inserting user...")
	passwordHash, err := hashPassword("password")
	if err != nil {
		fmt.Fprintf(os.Stderr, "hash password: %v\n", err)
		os.Exit(1)
	}
	_, err = pool.Exec(ctx,
		`INSERT INTO users (id, partner_id, email, password_hash, display_name, locale) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING`,
		devUserID, devPartnerID, "admin@acme-hosting.test", passwordHash, "Admin", "en")
	if err != nil {
		fmt.Fprintf(os.Stderr, "insert user: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("  Linking user to customer...")
	_, err = pool.Exec(ctx,
		`INSERT INTO customer_users (customer_id, user_id, permissions) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		devCustomerID, devUserID, []string{"*"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "link user: %v\n", err)
		os.Exit(1)
	}

	// --- Second partner (reseller) ---

	fmt.Println("  Inserting reseller partner (Bob's Web)...")
	_, err = pool.Exec(ctx,
		`INSERT INTO partners (id, brand_id, name, hostname, primary_color, status) VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (id) DO UPDATE SET hostname = EXCLUDED.hostname`,
		devPartner2ID, devBrandID, "Bob's Web", "bobs.massive-hosting.com", "145", "active")
	if err != nil {
		fmt.Fprintf(os.Stderr, "insert partner 2: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("  Inserting reseller customer...")
	_, err = pool.Exec(ctx,
		`INSERT INTO customers (id, partner_id, name, email, status) VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`,
		devCustomer2ID, devPartner2ID, "Bob's Client Inc", "info@bobs-client.test", "active")
	if err != nil {
		fmt.Fprintf(os.Stderr, "insert customer 2: %v\n", err)
		os.Exit(1)
	}

	passwordHash2, err := hashPassword("password")
	if err != nil {
		fmt.Fprintf(os.Stderr, "hash password 2: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  Inserting reseller user...")
	_, err = pool.Exec(ctx,
		`INSERT INTO users (id, partner_id, email, password_hash, display_name, locale) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING`,
		devUser2ID, devPartner2ID, "bob@bobs-web.test", passwordHash2, "Bob", "en")
	if err != nil {
		fmt.Fprintf(os.Stderr, "insert user 2: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("  Linking reseller user to customer...")
	_, err = pool.Exec(ctx,
		`INSERT INTO customer_users (customer_id, user_id, permissions) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		devCustomer2ID, devUser2ID, []string{"*"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "link user 2: %v\n", err)
		os.Exit(1)
	}

	// --- Products from YAML ---

	fmt.Println("  Seeding products from YAML...")
	if err := seedProducts(ctx, pool); err != nil {
		fmt.Fprintf(os.Stderr, "seed products: %v\n", err)
		os.Exit(1)
	}

	// --- Dynamic data: discover subscriptions from hosting API ---

	hostingURL := os.Getenv("HOSTING_API_URL")
	hostingKey := os.Getenv("HOSTING_API_KEY")

	if hostingURL != "" && hostingKey != "" {
		fmt.Println("  Syncing subscriptions from hosting API...")
		if err := syncSubscriptionsFromHosting(ctx, pool, hostingURL, hostingKey, devCustomerID, devBrandID); err != nil {
			fmt.Fprintf(os.Stderr, "  WARNING: hosting sync failed: %v\n", err)
			fmt.Fprintln(os.Stderr, "  Skipping subscription sync — control panel will work but show no hosting data.")
		}
	} else {
		fmt.Println("  WARNING: HOSTING_API_URL or HOSTING_API_KEY not set, skipping subscription sync.")
	}

	fmt.Println()
	fmt.Println("Seed complete!")
	fmt.Println()
	fmt.Println("  Partner: Acme Hosting (home.massive-hosting.com)")
	fmt.Println("    Login: admin@acme-hosting.test / password")
	fmt.Println()
	fmt.Println("  Partner: Bob's Web (bobs.massive-hosting.com)")
	fmt.Println("    Login: bob@bobs-web.test / password")
}

// seedProducts reads seeds/products.yaml and upserts rows into the products table.
func seedProducts(ctx context.Context, pool *pgxpool.Pool) error {
	// Resolve path relative to this source file so it works regardless of cwd.
	_, thisFile, _, _ := runtime.Caller(0)
	yamlPath := filepath.Join(filepath.Dir(thisFile), "products.yaml")

	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return fmt.Errorf("read products.yaml: %w", err)
	}

	var pf productsFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return fmt.Errorf("parse products.yaml: %w", err)
	}

	for _, p := range pf.Products {
		fmt.Printf("    Upserting product %s (%s)\n", p.ID, p.Name)
		_, err := pool.Exec(ctx,
			`INSERT INTO products (id, brand_id, name, description, modules, status)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (id) DO UPDATE SET
			   brand_id = EXCLUDED.brand_id,
			   name = EXCLUDED.name,
			   description = EXCLUDED.description,
			   modules = EXCLUDED.modules,
			   status = EXCLUDED.status,
			   updated_at = now()`,
			p.ID, p.BrandID, p.Name, p.Description, p.Modules, p.Status)
		if err != nil {
			return fmt.Errorf("upsert product %s: %w", p.ID, err)
		}
	}

	return nil
}

// syncSubscriptionsFromHosting discovers tenants and their resources from the
// hosting API and upserts corresponding customer_subscriptions rows.
func syncSubscriptionsFromHosting(ctx context.Context, pool *pgxpool.Pool, baseURL, apiKey, customerID, brandID string) error {
	client := &http.Client{Timeout: 10 * time.Second}

	// 1. Discover tenants for the customer.
	tenants, err := hostingGet[[]tenantEntry](ctx, client, baseURL, apiKey, "/tenants?customer_id="+customerID)
	if err != nil {
		return fmt.Errorf("list tenants: %w", err)
	}

	fmt.Printf("    Found %d tenant(s)\n", len(tenants))

	for _, t := range tenants {
		// 2. Get subscriptions for the tenant.
		subs, err := hostingGet[[]subscriptionEntry](ctx, client, baseURL, apiKey, fmt.Sprintf("/tenants/%s/subscriptions", t.ID))
		if err != nil {
			return fmt.Errorf("list subscriptions for tenant %s: %w", t.ID, err)
		}

		// 3. Upsert a subscription row per hosting subscription.
		for _, sub := range subs {
			// Look up the product_id by (brand_id, product_name).
			var productID string
			err := pool.QueryRow(ctx,
				`SELECT id FROM products WHERE brand_id = $1 AND name = $2`,
				brandID, sub.ProductName).Scan(&productID)
			if err != nil {
				return fmt.Errorf("lookup product %q for brand %s: %w", sub.ProductName, brandID, err)
			}

			fmt.Printf("    Upserting subscription %s (tenant %s, product %s)\n", sub.ID, t.ID, productID)
			_, err = pool.Exec(ctx,
				`INSERT INTO customer_subscriptions (id, customer_id, tenant_id, product_id, status, updated_at)
				 VALUES ($1, $2, $3, $4, $5, $6)
				 ON CONFLICT (id) DO UPDATE SET
				   tenant_id = EXCLUDED.tenant_id,
				   product_id = EXCLUDED.product_id,
				   status = EXCLUDED.status,
				   updated_at = EXCLUDED.updated_at`,
				sub.ID, customerID, t.ID, productID, sub.Status, time.Now())
			if err != nil {
				return fmt.Errorf("upsert subscription %s: %w", sub.ID, err)
			}
		}
	}

	return nil
}

// hostingGet makes a GET request to the hosting API and decodes the JSON response.
// For list endpoints, the hosting API wraps results in {"items": [...]} — we try
// to unwrap that automatically when T is a slice type.
func hostingGet[T any](ctx context.Context, client *http.Client, baseURL, apiKey, path string) (T, error) {
	var zero T

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return zero, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("GET %s: status %d", path, resp.StatusCode)
	}

	// Try to decode directly first. If T is a slice and the response is wrapped
	// in {"items": [...]}, fall back to unwrapping.
	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return zero, fmt.Errorf("decode response: %w", err)
	}

	var result T
	if err := json.Unmarshal(raw, &result); err == nil {
		return result, nil
	}

	// Try unwrapping from paginated envelope.
	var envelope struct {
		Items T `json:"items"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return zero, fmt.Errorf("decode response: %w", err)
	}
	return envelope.Items, nil
}

type tenantEntry struct {
	ID         string `json:"id"`
	CustomerID string `json:"customer_id"`
}

type subscriptionEntry struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	ProductName string `json:"name"`
	Status      string `json:"status"`
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	// Match @node-rs/argon2 defaults: m=65536, t=3, p=4, keyLen=32
	hash := argon2.IDKey([]byte(password), salt, 3, 65536, 4, 32)

	return fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=4$%s$%s",
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}
