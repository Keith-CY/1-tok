FROM mcr.microsoft.com/playwright:v1.55.0-noble

WORKDIR /app

COPY harness/e2e-screenshots/package.json /app/harness/e2e-screenshots/package.json
RUN cd /app/harness/e2e-screenshots && npm install --omit=dev
COPY harness/e2e-screenshots /app/harness/e2e-screenshots

WORKDIR /app/harness/e2e-screenshots
CMD ["node", "capture.mjs"]
