# راهنمای فارسی راه‌اندازی mrkvpn با Docker

این نسخه، **نسخه آزمایشی توسعه (Alpha)** است. بدون داشتن VPS یا محیط تست اختصاصی می‌توانی آن را Build کنی، کلید اختصاصی **سرور** بسازی، تنظیمات WireGuard را اعتبارسنجی کنی و فایل پیکربندی تولید کنی. این کارها به معنای داشتن VPN کامل یا محافظت در برابر نشت ترافیک نیست.

## پیش‌نیاز

- Git و Docker Engine یا Docker Desktop به همراه Compose v2
- اینترنت برای دریافت Imageها و پکیج‌ها در اولین Build
- نیازی به نصب Go، WireGuard یا ابزارهای شبکه روی سیستم توسعه نیست
- در Windows بهتر است داخل WSL2 و مسیر فایل‌سیستم لینوکسی (نه `/mnt/c`) کار کنی؛ چون کلیدهای خصوصی باید دسترسی واقعی `0600` داشته باشند

## ۱. دریافت پروژه و ساخت ابزارها

```bash
git clone https://github.com/sysmrk/mrkvpn.git
cd mrkvpn
docker compose --profile tools build
```

این مرحله فقط Image ابزارها را می‌سازد و هیچ پورت VPNای باز نمی‌کند.

## ۲. راه‌اندازی اولیه بدون شبکه و بدون VPS

```bash
docker compose --profile tools run --rm init
```

خروجی‌های محلی:

- `.local/secrets/gateway.key`: **کلید خصوصی Gateway؛ محرمانه**
- `.local/secrets/gateway.pub`: کلید عمومی Gateway
- `.local/config/gateway.json`: تنظیمات Gateway با فهرست کاربران خالی
- `.local/output/`: مسیر فایل‌های تولیدشده

اجرای مجدد `init` کلید موجود را بازنویسی نمی‌کند. در این مرحله **هیچ کلید خصوصی کلاینتی** ساخته نمی‌شود.

## ۳. بررسی و تولید تنظیمات

```bash
docker compose --profile tools run --rm cli gateway check -config /config/gateway.json
docker compose --profile tools run --rm cli gateway render -config /config/gateway.json -out /output/wg0.conf
```

دستور `check` فقط بررسی می‌کند؛ `render` فایل `.local/output/wg0.conf` می‌سازد و اگر وجود داشته باشد از بازنویسی جلوگیری می‌کند. این فایل شامل **کلید خصوصی Gateway** است و نباید وارد Git یا Log شود.

برای افزودن کاربر، کلید خصوصی را **روی خود دستگاه کاربر** بساز و فقط Public Key آن را در `.local/config/gateway.json` و قسمت `peers` وارد کن؛ هر دستگاه باید IP اختصاصی `/32` داشته باشد. بعد از ویرایش `check` را مجدد اجرا کن و برای تولید نسخه جدید، فایل قدیمی را ابتدا با تصمیم آگاهانه و پس از توقف Gateway آزمایشگاهی بکاپ/جابه‌جا کن.

## ۴. Gateway واقعی آزمایشگاهی — فقط وقتی محیط Linux آماده شد

این دستور اختیاری است، پورت UDP 51820 را منتشر می‌کند و به WireGuard Kernel و مجوز `NET_ADMIN` نیاز دارد:

```bash
docker compose -f compose.yaml -f compose.lab.yaml --profile lab up -d --build gateway
```

مشاهده وضعیت و توقف:

```bash
docker compose -f compose.yaml -f compose.lab.yaml --profile lab ps
docker compose -f compose.yaml -f compose.lab.yaml --profile lab logs --follow gateway
docker compose -f compose.yaml -f compose.lab.yaml --profile lab down
```

**هشدار:** این Gateway تنها سمت سرور را برای آزمایش WireGuard آماده می‌کند. هنوز برنامه کلاینت mrkvpn، Kill Switch، کنترل DNS/IPv6 Leak، احراز هویت و پنل کامل کاربران، QUIC و Cloudflare Relay پیاده‌سازی نشده‌اند. فعلاً برای ارتباط حساس یا انتشار عمومی قابل اتکا نیست.

## ۵. ابزارهای جانبی و تست خودکار

برای ترمینال عیب‌یابی داخل یک کانتینر جدا:

```bash
docker compose --profile diagnostics run --rm diag
```

ابزارهای `wg`، `ip`، `iptables`، `nft`، `curl`، `dig`، `jq`، `ping`، `tcpdump` و `openssl` در Image ابزارها موجود هستند. شبکه این Toolbox شبکه میزبان نیست.

برای توسعه و بررسی بدون VPS:

```bash
make test
make lint
docker compose config --quiet
```

اگر `make` در Windows نصب نیست، اجرای GitHub Actions و دستورات مستقیم Docker Compose کافی است؛ لازم نیست برای همین موضوع Make نصب کنی.

## امنیت مهم

- مسیر `.local/` با `.gitignore` از Commit کنار گذاشته شده، ولی **این جایگزین حفاظت واقعی از کلید نیست**.
- کسی که به Docker daemon یا فایل‌های کلید روی میزبان دسترسی دارد می‌تواند کلیدها را بخواند.
- کانتینر ابزارها شبکه ندارد و بدون Capability اضافی اجرا می‌شود؛ Gateway آزمایشگاهی جداست.
- در حالت پیش‌فرض هیچ Gatewayای اجرا یا روی اینترنت منتشر نمی‌شود.
- Cloudflare در معماری اختیاری است، اما در نسخه فعلی هنوز پیاده‌سازی نشده.

راهنمای جزئیات انگلیسی: [Docker getting started](docker-getting-started.md) و [محدودیت‌های امنیتی](security.md).
