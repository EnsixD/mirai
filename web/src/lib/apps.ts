// Names for where a connection goes: the app or site behind a host name or an address.
// The connection log groups by them, so a card reads "TikTok", not forty CDN hosts.

export type ServiceKind = "dns" | "push" | "check" | "ads";

export type Target = {
  /** What lines are grouped by. */
  key: string;
  /** The app's or site's name; for a service kind and for "ip" the caller has the words. */
  name: string;
  kind: "app" | "site" | "service" | "ip";
  service?: ServiceKind;
};

// App → the domains it talks to (a subdomain matches too; the longest match wins).
const APPS: Record<string, string> = {
  Telegram: "telegram.org t.me telegram.me tdesktop.com telegra.ph telesco.pe telegram-cdn.org cdn-telegram.org tg.dev",
  WhatsApp: "whatsapp.net whatsapp.com wa.me",
  Instagram: "instagram.com cdninstagram.com ig.me",
  Facebook: "facebook.com fbcdn.net fb.com facebook.net fbsbx.com fb.me messenger.com m.me",
  Threads: "threads.net threads.com",
  TikTok:
    "tiktok.com tiktokv.com tiktokv.us tiktokw.us tiktokcdn.com tiktokcdn-us.com tiktokcdn-eu.com ttwstatic.com ttlivecdn.com byteoversea.com byteoversea.net ibytedtos.com ibyteimg.com muscdn.com musical.ly isnssdk.com bytedance.com byteimg.com bytegecko.com tiktokglobalshop.com",
  YouTube: "youtube.com googlevideo.com ytimg.com youtu.be youtubei.googleapis.com youtube-nocookie.com yt3.ggpht.com youtube.googleapis.com",
  Google:
    "google.com google.ru gstatic.com googleapis.com googleusercontent.com gvt1.com gvt2.com gvt3.com 1e100.net android.com ggpht.com google.com.tr google.de google.co.uk g.co goo.gl withgoogle.com",
  "Google Play": "play.google.com play.googleapis.com play-fe.googleapis.com android.clients.google.com",
  Gmail: "gmail.com mail.google.com googlemail.com",
  "Google Maps": "maps.google.com maps.googleapis.com maps.gstatic.com",
  Gemini: "gemini.google.com generativelanguage.googleapis.com bard.google.com",
  "X (Twitter)": "twitter.com x.com twimg.com t.co",
  Discord: "discord.com discord.gg discordapp.com discordapp.net discord.media",
  Netflix: "netflix.com nflxvideo.net nflximg.net nflxso.net nflxext.com",
  Spotify: "spotify.com scdn.co spotifycdn.com",
  ChatGPT: "openai.com chatgpt.com oaistatic.com oaiusercontent.com",
  Claude: "claude.ai anthropic.com claude.com",
  Twitch: "twitch.tv ttvnw.net jtvnw.net",
  Steam: "steampowered.com steamcommunity.com steamstatic.com steamcontent.com steamusercontent.com steam-chat.com steamserver.net",
  Apple: "apple.com icloud.com mzstatic.com apple-cloudkit.com icloud-content.com cdn-apple.com aaplimg.com apple.news me.com",
  Microsoft: "microsoft.com live.com office.com windows.com windowsupdate.com office365.com outlook.com msn.com microsoftonline.com windows.net office.net live.net",
  Bing: "bing.com bing.net",
  Skype: "skype.com skypeassets.com",
  LinkedIn: "linkedin.com licdn.com",
  GitHub: "github.com githubusercontent.com githubassets.com github.io",
  Reddit: "reddit.com redd.it redditstatic.com redditmedia.com",
  Snapchat: "snapchat.com sc-cdn.net snap.com snapkit.com",
  Pinterest: "pinterest.com pinimg.com",
  Viber: "viber.com",
  Signal: "signal.org whispersystems.org",
  Zoom: "zoom.us zoom.com",
  Wikipedia: "wikipedia.org wikimedia.org",
  Amazon: "amazon.com media-amazon.com amazon.de ssl-images-amazon.com primevideo.com",
  "Epic Games": "epicgames.com unrealengine.com epicgames.dev fortnite.com",
  "Riot Games": "riotgames.com leagueoflegends.com riotcdn.net playvalorant.com",
  Roblox: "roblox.com rbxcdn.com",
  Supercell: "supercell.com supercell.net brawlstars.com clashofclans.com clashroyale.com",
  HoYoverse: "hoyoverse.com mihoyo.com hoyolab.com",
  PlayStation: "playstation.com playstation.net sonyentertainmentnetwork.com",
  Xbox: "xbox.com xboxlive.com",
  "Battle.net": "battle.net blizzard.com",
  FACEIT: "faceit.com faceit-cdn.net",
  VK: "vk.com vk.ru userapi.com vk-cdn.net vkuser.net vkvideo.ru vk.me vkuseraudio.net mycdn.me",
  Yandex: "yandex.ru yandex.net yandex.com ya.ru yastatic.net yandex.kz yandex.by yandexcloud.net",
  "Mail.ru": "mail.ru imgsmail.ru",
  OK: "ok.ru odnoklassniki.ru",
  Dzen: "dzen.ru",
  Rutube: "rutube.ru",
  Kinopoisk: "kinopoisk.ru hd.kinopoisk.ru",
  Ozon: "ozon.ru ozone.ru",
  Wildberries: "wildberries.ru wb.ru wbbasket.ru wbstatic.net",
  Avito: "avito.ru avito.st",
  Sber: "sberbank.ru sber.ru online.sberbank.ru",
  "T-Bank": "tbank.ru tinkoff.ru tcsbank.ru",
  Gosuslugi: "gosuslugi.ru",
  Cloudflare: "cloudflare.com cloudflare.net cloudflareinsights.com cdnjs.com",
  "Amazon AWS": "amazonaws.com cloudfront.net awsstatic.com",
  Akamai: "akamai.net akamaihd.net akamaized.net akamaiedge.net edgekey.net edgesuite.net",
  Fastly: "fastly.net fastlylb.net",
};

// What a phone does by itself, whatever its owner is doing: the log folds it away.
const SERVICES: Record<ServiceKind, string> = {
  dns: "dns.google dns.google.com cloudflare-dns.com one.one.one.one dns.quad9.net dns.adguard.com dns.adguard-dns.com dns.nextdns.io dns.yandex.ru",
  push: "mtalk.google.com alt1-mtalk.google.com alt2-mtalk.google.com alt3-mtalk.google.com alt4-mtalk.google.com alt5-mtalk.google.com alt6-mtalk.google.com alt7-mtalk.google.com alt8-mtalk.google.com push.apple.com mobile-gtalk.l.google.com firebaseinstallations.googleapis.com fcm.googleapis.com fcmtoken.googleapis.com",
  check:
    "connectivitycheck.gstatic.com connectivitycheck.android.com clients3.google.com clients4.google.com captive.apple.com msftconnecttest.com msftncsi.com detectportal.firefox.com time.android.com time.google.com time.apple.com time.windows.com pool.ntp.org",
  ads: "doubleclick.net googlesyndication.com googleadservices.com google-analytics.com googletagmanager.com googletagservices.com app-measurement.com crashlytics.com firebaselogging-pa.googleapis.com firebase-settings.crashlytics.com appsflyer.com appsflyersdk.com adjust.com branch.io applovin.com unityads.unity3d.com mc.yandex.ru an.yandex.ru ads.vk.com adservice.google.com sentry.io app-analytics-services.com",
};

// Addresses that are an app even without a name: these apps connect to bare IPs.
const NETS: [cidr: string, name: string][] = [
  ["149.154.160.0/20", "Telegram"],
  ["91.108.4.0/22", "Telegram"],
  ["91.108.8.0/21", "Telegram"],
  ["91.108.16.0/21", "Telegram"],
  ["91.108.56.0/22", "Telegram"],
  ["91.105.192.0/23", "Telegram"],
  ["95.161.64.0/20", "Telegram"],
  ["185.76.151.0/24", "Telegram"],
  ["157.240.0.0/16", "Meta"],
  ["31.13.24.0/21", "Meta"],
  ["31.13.64.0/18", "Meta"],
  ["129.134.0.0/16", "Meta"],
  ["163.70.128.0/17", "Meta"],
  ["179.60.192.0/22", "Meta"],
  ["185.60.216.0/22", "Meta"],
  ["66.220.144.0/20", "Meta"],
  ["69.63.176.0/20", "Meta"],
  ["69.171.224.0/19", "Meta"],
];

const DNS_IPS = new Set(["8.8.8.8", "8.8.4.4", "1.1.1.1", "1.0.0.1", "9.9.9.9", "149.112.112.112", "77.88.8.8", "77.88.8.1", "94.140.14.14", "94.140.15.15", "208.67.222.222", "208.67.220.220", "2001:4860:4860::8888", "2001:4860:4860::8844", "2606:4700:4700::1111", "2606:4700:4700::1001"]);

const bySuffix = new Map<string, Target>();
for (const [name, domains] of Object.entries(APPS)) {
  for (const d of domains.split(" ")) bySuffix.set(d, { key: name, name, kind: "app" });
}
for (const [kind, domains] of Object.entries(SERVICES) as [ServiceKind, string][]) {
  for (const d of domains.split(" ")) bySuffix.set(d, { key: `~${kind}`, name: kind, kind: "service", service: kind });
}

const service = (kind: ServiceKind): Target => ({ key: `~${kind}`, name: kind, kind: "service", service: kind });

function v4(ip: string): number | null {
  const p = ip.split(".");
  if (p.length !== 4) return null;
  let n = 0;
  for (const part of p) {
    if (!/^\d{1,3}$/.test(part) || Number(part) > 255) return null;
    n = n * 256 + Number(part);
  }
  return n;
}

const nets = NETS.map(([cidr, name]) => {
  const [base, bits] = cidr.split("/");
  const size = 2 ** (32 - Number(bits));
  const from = v4(base!)!;
  return { from, to: from + size, name };
});

// Second-level zones under a country's domain: the site is one label further.
const SECOND_LEVEL = new Set(["co", "com", "net", "org", "gov", "edu", "ac"]);

function siteOf(host: string): string {
  const labels = host.split(".");
  if (labels.length <= 2) return host;
  const tld = labels[labels.length - 1]!;
  const take = tld.length === 2 && SECOND_LEVEL.has(labels[labels.length - 2]!) ? 3 : 2;
  return labels.slice(-take).join(".");
}

/** Splits "host:port" (an IPv6 host comes in brackets). */
export function splitDest(dest: string): { host: string; port: number } {
  const i = dest.lastIndexOf(":");
  if (i < 0) return { host: dest, port: 0 };
  return { host: dest.slice(0, i).replace(/^\[|\]$/g, ""), port: Number(dest.slice(i + 1)) || 0 };
}

/** Whose traffic a connection is: by the site's name when there is one, else by the address. */
export function classify(name: string, addr: string, port: number): Target {
  if (port === 53 || port === 853 || DNS_IPS.has(addr)) return service("dns");
  if (port === 123) return service("check");
  const host = name.toLowerCase().replace(/\.$/, "");
  if (host && v4(host) === null && !host.includes(":")) {
    const labels = host.split(".");
    for (let i = 0; i < labels.length - 1; i++) {
      const hit = bySuffix.get(labels.slice(i).join("."));
      if (hit) return hit;
    }
    const site = siteOf(host);
    return { key: site, name: site, kind: "site" };
  }
  const n = v4(addr);
  if (n !== null) {
    const net = nets.find((r) => n >= r.from && n < r.to);
    if (net) return { key: net.name, name: net.name, kind: "app" };
  }
  return { key: "~ip", name: addr, kind: "ip" };
}
