#!/bin/sh
set -eu

python migrate.py || echo "[entrypoint] migrate failed or skipped"

gen_reports() {
  echo "[entrypoint] Generating user_service coverage HTML..."
  PYTHON=python3; command -v python3 >/dev/null 2>&1 || PYTHON=python
	"$PYTHON" - <<'PY'
import os, sys, subprocess, time
data_file = os.environ.get('COVERAGE_FILE', '/coverage/.coverage.user_service')
rcfile = '.coveragerc'
html_dir = os.environ.get('HTML_DIR', '/svc_coverage/htmlcov')
cov_dir = os.path.dirname(data_file) or '/coverage'
prefix = os.path.basename(data_file)
files = []
for _ in range(10):
	files = [os.path.join(cov_dir, f) for f in os.listdir(cov_dir) if f.startswith(prefix) and f != prefix]
	if files or os.path.exists(data_file):
		break
	time.sleep(0.5)
combined = data_file + '.combined'
try:
	if files:
		cmd = [sys.executable, '-m', 'coverage', 'combine', '--data-file', combined]
		cmd += files
		subprocess.check_call(cmd)
		used = combined
	else:
		used = data_file if os.path.exists(data_file) else None
	if not used:
		print('[entrypoint] No coverage data found for user_service in', cov_dir)
	else:
		os.makedirs(html_dir, exist_ok=True)
		subprocess.check_call([sys.executable, '-m', 'coverage', 'report', '-m', '--data-file', used, '--rcfile', rcfile])
		subprocess.check_call([sys.executable, '-m', 'coverage', 'html', '-d', html_dir, '--data-file', used, '--rcfile', rcfile])
		print('[entrypoint] HTML written to', os.path.join(html_dir, 'index.html'))
except Exception as e:
	print('[entrypoint] coverage combine/report/html failed:', e)
PY
}

if [ -n "${COVERAGE:-}" ]; then
	echo "[entrypoint] Running under coverage"
	export COVERAGE_FILE=${COVERAGE_FILE:-/coverage/.coverage.user_service}
	python -m coverage run --rcfile=.coveragerc app.py &
	APP_PID=$!
	on_term() {
		echo "[entrypoint] Caught stop signal; stopping app..."
		kill -TERM "$APP_PID" 2>/dev/null || true
		wait "$APP_PID" || true
		sleep 1
		gen_reports || true
		exit 0
	}
	trap on_term INT TERM
	wait "$APP_PID" || true
	sleep 1
	gen_reports || true
else
	python app.py
fi
