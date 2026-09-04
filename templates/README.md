# Email and landing-page templates for Gophish

Authorized security-awareness use only.

## Layout

| Path | Source |
|------|--------|
| `templates/insec/` | Original IT Support / HR / voicemail pack for `itsupport.insec.in` |
| `templates/vendor/hailbytes/` | [HailBytes/gophish-training-templates](https://github.com/HailBytes/gophish-training-templates) (MPL-2.0) |
| `templates/vendor/linksec/` | [LinkSec/phishing-templates](https://github.com/LinkSec/phishing-templates) (emails) |
| `templates/vendor/piyush27pawar/` | [piyush27pawar/GoPhish-Templates](https://github.com/piyush27pawar/GoPhish-Templates) |

Keep each vendor `LICENSE` / `README` with those files.

## Import on the VPS

```bash
cd /opt/gophish && git pull origin main
apt-get install -y sqlite3 python3
export GOPHISH_API_KEY=$(sqlite3 /opt/gophish/runtime/gophish.db "SELECT api_key FROM users WHERE username='admin';")
python3 /opt/gophish/scripts/import-templates.py
```

Templates appear in Gophish under **Email Templates** and **Landing Pages**. Re-running skips names that already exist.

English only: HailBytes Spanish/Portuguese packs are not imported. To drop them from the repo and from Gophish if they were already imported:

```bash
cd /opt/gophish && git pull origin main
export GOPHISH_API_KEY=$(sqlite3 /opt/gophish/runtime/gophish.db "SELECT api_key FROM users WHERE username='admin';")
python3 /opt/gophish/scripts/keep-english-templates.py --gophish
```

Optional education redirect (default is StaySafeOnline):

```bash
export GOPHISH_REDIRECT_URL="https://awarechck.com"
python3 /opt/gophish/scripts/import-templates.py
```
