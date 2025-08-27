import os
import glob
import time
import mysql.connector

DB_HOST = os.environ.get('DB_HOST', 'localhost')
DB_USER = os.environ.get('DB_USER', 'user')
DB_PASSWORD = os.environ.get('DB_PASSWORD', 'password')
DB_NAME = os.environ.get('DB_NAME', 'user_db')

def get_conn(retries: int = 30, delay: float = 1.0):
    last_err = None
    for _ in range(retries):
        try:
            return mysql.connector.connect(
                host=DB_HOST,
                user=DB_USER,
                password=DB_PASSWORD,
                database=DB_NAME,
            )
        except Exception as e:
            last_err = e
            time.sleep(delay)
    raise last_err

def ensure_migrations_table(cur):
    cur.execute(
        """
        CREATE TABLE IF NOT EXISTS schema_migrations (
            id INT AUTO_INCREMENT PRIMARY KEY,
            version VARCHAR(64) NOT NULL UNIQUE,
            applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )
        """
    )

def applied_versions(cur):
    cur.execute("SELECT version FROM schema_migrations")
    return {row[0] for row in cur.fetchall()}

def apply_sql(cur, sql):
    for stmt in [s.strip() for s in sql.split(';') if s.strip()]:
        cur.execute(stmt)
        try:
            if getattr(cur, 'with_rows', False):
                try:
                    cur.fetchall()
                except Exception:
                    pass
        except Exception:
            pass

def main():
    conn = get_conn()
    cur = conn.cursor()
    ensure_migrations_table(cur)
    done = applied_versions(cur)
    files = sorted(glob.glob(os.path.join(os.path.dirname(__file__), 'migrations', '*.sql')))
    for f in files:
        version = os.path.basename(f).split('_', 1)[0]
        if version in done:
            continue
        with open(f, 'r', encoding='utf-8') as fh:
            sql = fh.read()
        apply_sql(cur, sql)
        cur.execute("INSERT INTO schema_migrations (version) VALUES (%s)", (version,))
        conn.commit()
        print(f"Applied migration {version}: {f}")
    cur.close(); conn.close()

if __name__ == '__main__':
    main()
