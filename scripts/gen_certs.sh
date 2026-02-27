#!/bin/bash

mkdir -p certs

# Генерируем приватный ключ (2048 бит)
openssl genpkey -algorithm RSA -out certs/private.pem -pkeyopt rsa_keygen_bits:2048

# Извлекаем публичный ключ из приватного
openssl rsa -pubout -in certs/private.pem -out certs/public.pem

echo "RSA keys generated successfully in ./certs directory."
