import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Counter } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const CHAT_COUNT = Number(__ENV.CHAT_COUNT || 1000);
const LINKS_PER_CHAT = Number(__ENV.LINKS_PER_CHAT || 100);
const SLEEP_SECONDS = Number(__ENV.SLEEP_SECONDS || 0.05);

const getLinksDuration = new Trend('get_links_duration_ms', true);
const postLinksDuration = new Trend('post_links_duration_ms', true);
const deleteLinksDuration = new Trend('delete_links_duration_ms', true);

const get200 = new Counter('get_200');

const post200 = new Counter('post_200');
const post201 = new Counter('post_201');
const post409 = new Counter('post_409');

const delete200 = new Counter('delete_200');
const delete204 = new Counter('delete_204');
const delete404 = new Counter('delete_404');

const status500 = new Counter('http_500');
const status502 = new Counter('http_502');
const status504 = new Counter('http_504');
const status502504 = new Counter('http_502_504');

export const options = {
    scenarios: {
        list_mix: {
            executor: 'ramping-vus',
            startVUs: 1,
            stages: [
                { duration: '1m', target: Number(__ENV.VUS || 8) },
                { duration: __ENV.STAGE_DURATION || '5m', target: Number(__ENV.VUS || 8) },
                { duration: '30s', target: 0 },
            ],
            gracefulRampDown: '30s',
        },
    },
    thresholds: {
        http_req_failed: ['rate<0.02'],
    },
};

function randomChatId() {
    return 1 + Math.floor(Math.random() * CHAT_COUNT);
}

function headers(chatId) {
    return {
        'Content-Type': 'application/json',
        'Tg-Chat-Id': String(chatId),
    };
}

function countCommonStatuses(res) {
    status500.add(res.status === 500 ? 1 : 0);
    status502.add(res.status === 502 ? 1 : 0);
    status504.add(res.status === 504 ? 1 : 0);
    status502504.add(res.status === 502 || res.status === 504 ? 1 : 0);
}

function countGetStatuses(res) {
    get200.add(res.status === 200 ? 1 : 0);
    countCommonStatuses(res);
}

function countPostStatuses(res) {
    post200.add(res.status === 200 ? 1 : 0);
    post201.add(res.status === 201 ? 1 : 0);
    post409.add(res.status === 409 ? 1 : 0);
    countCommonStatuses(res);
}

function countDeleteStatuses(res) {
    delete200.add(res.status === 200 ? 1 : 0);
    delete204.add(res.status === 204 ? 1 : 0);
    delete404.add(res.status === 404 ? 1 : 0);
    countCommonStatuses(res);
}

export default function () {
    const chatId = randomChatId();

    const opRoll = Math.floor(Math.random() * 101);

    if (opRoll < 100) {
        const res = http.get(`${BASE_URL}/links`, {
            headers: headers(chatId),
        });

        getLinksDuration.add(res.timings.duration);
        countGetStatuses(res);

        check(res, {
            'GET /links status 200': (r) => r.status === 200,
        });
    } else {
        const linkNum = 1 + Math.floor(Math.random() * LINKS_PER_CHAT);
        const link = `https://example.com/${chatId}/${linkNum}`;

        const postRes = http.post(
            `${BASE_URL}/links`,
            JSON.stringify({
                link,
                tags: ['perf'],
                filters: [],
            }),
            {
                headers: headers(chatId),
            },
        );

        postLinksDuration.add(postRes.timings.duration);
        countPostStatuses(postRes);

        check(postRes, {
            'POST /links status 2xx/409': (r) => [200, 201, 409].includes(r.status),
        });

        const delRes = http.del(
            `${BASE_URL}/links`,
            JSON.stringify({ link }),
            {
                headers: headers(chatId),
            },
        );

        deleteLinksDuration.add(delRes.timings.duration);
        countDeleteStatuses(delRes);

        check(delRes, {
            'DELETE /links status 2xx/404': (r) => [200, 204, 404].includes(r.status),
        });
    }

    sleep(SLEEP_SECONDS);
}