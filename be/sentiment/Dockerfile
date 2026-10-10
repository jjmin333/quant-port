# 뉴스 제목 감성 분류기 — CPU 전용. 이유와 모델은 app.py 머리글.
FROM python:3.12-slim

WORKDIR /app
COPY requirements.txt .
# CPU 휠만 받는다(CUDA 휠은 수 GB). ARM64(OCI)에서도 같은 인덱스가 aarch64 휠을 준다.
RUN pip install --no-cache-dir --index-url https://download.pytorch.org/whl/cpu "torch>=2.6,<3" \
 && pip install --no-cache-dir "transformers>=4.46,<5"
COPY app.py .

# 모델은 첫 기동 때 HF에서 받아 볼륨(/root/.cache)에 둔다 — 번역기와 같은 방식.
ENV HF_HOME=/root/.cache/huggingface
EXPOSE 8200
CMD ["python", "-u", "app.py"]
