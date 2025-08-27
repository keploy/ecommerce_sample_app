import os, glob
import mysql.connector

DB_HOST = os.environ.get('DB_HOST', 'localhost')
DB_USER = os.environ.get('DB_USER', 'user')
DB_PASSWORD = os.environ.get('DB_PASSWORD', 'password')
DB_NAME = os.environ.get('DB_NAME', 'order_db')

def conn():
    return mysql.connector.connect(host=DB_HOST, user=DB_USER, password=DB_PASSWORD, database=DB_NAME)

def main():
    c = conn(); cur = c.cursor()
    cur.execute("CREATE TABLE IF NOT EXISTS schema_migrations (id INT AUTO_INCREMENT PRIMARY KEY, version VARCHAR(64) UNIQUE, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)")
    cur.execute("SELECT version FROM schema_migrations"); done = {r[0] for r in cur.fetchall()}
    for f in sorted(glob.glob(os.path.join(os.path.dirname(__file__), 'migrations', '*.sql'))):
        v = os.path.basename(f).split('_',1)[0]
        if v in done: continue
        sql = open(f,'r',encoding='utf-8').read()
        for stmt in [s.strip() for s in sql.split(';') if s.strip()]:
            cur.execute(stmt)
            # Consume any result rows to avoid "Unread result found" when next statement executes
            try:
                if getattr(cur, 'with_rows', False):
                    try:
                        cur.fetchall()
                    except Exception:
                        pass
            except Exception:
                pass
        cur.execute("INSERT INTO schema_migrations (version) VALUES (%s)", (v,)); c.commit(); print('Applied', v)
    cur.close(); c.close()

if __name__=='__main__': main()
