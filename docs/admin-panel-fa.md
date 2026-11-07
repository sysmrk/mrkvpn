# پنل مدیریتی mrkvpn — راهنمای نصب و استفاده (Alpha)

**وضعیت:** پنل، API و ذخیره‌سازی داده پیاده‌سازی اولیه شده‌اند. سیستم مدیریت **Desired State** است و هنوز به WireGuard Gateway واقعی متصل نیست. **Suspend/Revocation در این پنل فعلاً اتصال یک کاربر یا دستگاه را در VPN قطع نمی‌کند.**

## پیش‌نیاز

- Docker Engine یا Docker Desktop و Docker Compose v2
- پورت آزاد **8787 روی localhost**
- برای توسعه و آزمایش پنل **نیازی به VPS، شبکه WireGuard یا نصب Go روی میزبان نیست**.
- رمزنگاری ارتباط مرورگر و API در Docker فعلاً HTTP روی Loopback است. هیچ پورتی را روی آدرس عمومی منتشر نکنید.

تا قبل از ادغام PR پنل، Branch زیر را Clone کنید:

```bash
git clone --branch feat/control-plane-admin-foundation --single-branch https://github.com/sysmrk/mrkvpn.git
cd mrkvpn
```

## نصب و اجرای پنل

```bash
docker compose -f compose.yaml -f compose.control.yaml --profile admin up -d --build control
docker compose -f compose.yaml -f compose.control.yaml --profile admin ps
```

سپس در مرورگر سیستم میزبان باز کنید:

**http://127.0.0.1:8787**

هنگام اجرای اولیه، کلید Owner با ۲۵۶ بیت تصادفی داخل Docker Volume ساخته می‌شود و فقط SHA-256 آن در State ذخیره می‌شود. مقدار اصلی در `/data/bootstrap.token` با Permission `0600` باقی می‌ماند.

برای **دریافت کلید روی دستگاه شخصی و در محیط امن**:

```bash
docker compose -f compose.yaml -f compose.control.yaml --profile admin exec -T control cat /data/bootstrap.token
```

**مهم:** خروجی یک Secret واقعی است. آن را داخل Git، Issue، Screenshot، Log، فایل متنی بدون رمزنگاری یا پیام‌رسان قرار ندهید. این دستور کلید را در ترمینال نمایش می‌دهد. بعد از ورود، بهتر است یک Token جدید Owner با تاریخ انقضا تعریف کنید، آن را در Password Manager حفظ کنید و Token اولیه را Revoke کنید. فایل `bootstrap.token` خودکار حذف نمی‌شود و حذف امن آن بعد از انتقال دسترسی به مالک جدید باید در فرآیند عملیاتی انجام شود. **آخرین Token معتبر Owner را نمی‌توان Revoke کرد.**

ورود به پنل از طریق کادر Access Token است. کلید در `localStorage` یا Cookie ذخیره نمی‌شود؛ بعد از Refresh یا بستن صفحه باید دوباره وارد شود.

## ویژگی‌های فعال فعلی

| بخش | امکانات پیاده‌سازی‌شده | نکته |
|---|---|---|
| Overview | شمارش واقعی Users/Devices/Gateways و Recent Audit | این‌ها وضعیت ارتباط واقعی VPN نیستند |
| Users | ایجاد، تعلیق/فعال‌سازی، تاریخ انقضا، محدودیت تعداد دستگاه | هنوز اعمال مستقیم روی Gateway وجود ندارد |
| Devices | ثبت نام دستگاه، پلتفرم، Public Key و Revoke | کلید خصوصی Client نباید وارد پنل شود |
| Gateways | ثبت IP/Port و Enable/Disable | وضعیت مدیریتی است، Health Check شبکه واقعی نیست |
| Policies | Automatic/Preferred/Manual، فهرست Transport و DNS Desired Policy | فعلاً روی Client اعمال نمی‌شود |
| Audit | رویدادهای تغییرات مدیریتی و Actor Token ID | Audit محلی است؛ SIEM و امضای زنجیره‌ای بعداً |
| Access Tokens | ایجاد Token تصادفی با نقش، Expiration و Revoke | فعلاً MFA، Passkey و SSO نداریم |

### نقش‌ها

| نقش | مشاهده اطلاعات | مدیریت User/Device/Gateway/Policy | مدیریت Token |
|---|---|---|---|
| Owner | بله | بله | بله |
| Admin | بله | بله | خیر |
| Operator | بله | خیر، فعلاً Read-only | خیر |
| Auditor | بله | خیر | خیر |

Token جدید فقط یک مرتبه در پاسخ API و پنل نشان داده می‌شود. Tokenهای خام در State نگهداری نمی‌شوند؛ اما فایل اولیه `bootstrap.token` یک استثنای روشن و مستند است.

## ذخیره‌سازی و پایداری

مخزن داده در **Named Docker Volume** به نام `mrkvpn_control-data` ذخیره می‌شود. State به شکل JSON با Schema Version و Atomic Rename نوشته می‌شود. Data directory و فایل‌ها دارای مجوزهای محدود هستند. یک File Lock مانع اجرای هم‌زمان دو Writer روی یک Volume می‌شود.

```bash
docker compose -f compose.yaml -f compose.control.yaml --profile admin logs --tail=100 control
docker compose -f compose.yaml -f compose.control.yaml --profile admin restart control
docker compose -f compose.yaml -f compose.control.yaml --profile admin down
```

`down` بدون `-v` داده‌ها را حفظ می‌کند. **هرگز برای توقف روزمره از `down -v` استفاده نکنید؛ این دستور Volume و کل داده‌ها را حذف می‌کند.**

این نسخه برای یک Instance و گروه محدود مناسب است؛ PostgreSQL، Migration رسمی، Backup رمزنگاری‌شده و HA پیش‌نیاز تبدیل آن به نسخه سازمانی خواهند بود.

## دسترسی از یک سیستم دیگر

پورت کانتینر عمداً فقط روی `127.0.0.1:8787` میزبان Publish می‌شود. اگر پنل روی سرور راه‌اندازی شد، بدون بازکردن پورت عمومی از SSH Port Forwarding استفاده کنید:

```bash
ssh -L 8787:127.0.0.1:8787 user@server
```

و روی سیستم خودتان `http://127.0.0.1:8787` را باز کنید. برای انتشار سازمانی به HTTPS، OIDC/SSO، MFA، RBAC کامل، Reverse Proxy و ارزیابی امنیتی مستقل نیاز داریم؛ **فعلاً انتشار عمومی توصیه نمی‌شود**.

## وضعیت واقعی سرویس

سلامت API: `http://127.0.0.1:8787/healthz`. پاسخ OK فقط به معنای فعال بودن HTTP Server است، **نه فعال بودن WireGuard، معتبر بودن Policy یا برقرار بودن اتصال کاربران**.

## نقشه راه بعدی پنل

1. Auth واقعی کاربران مدیریتی با Passkey/WebAuthn، MFA و OIDC؛ Recovery و محدودسازی نشست‌ها.
2. Enrollment امن دستگاه همراه با Challenge، Proof-of-Possession، صدور Profile و Key Rotation.
3. Gateway Agent با mTLS، Policy Signature، Versioned Apply/Ack/Rollback و Revoke عملی.
4. Active Sessions، Connection Telemetry، Transport Health و Alert بدون ثبت مقصدهای وب‌گردی.
5. Groups، Quotas، Expiration، Access Policies، Device Compliance و مدیریت چند Gateway.
6. PostgreSQL، Migration، Backup/Restore، HA، SIEM Export و Audit Tamper Evidence.
7. محدودسازی Rate، Access Review، Security Headers/Reverse Proxy، تست نفوذ و Release Signing.

برای قرارداد API و مدل داده به [control-plane-api.md](control-plane-api.md) مراجعه کنید.
