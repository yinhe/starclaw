#!/bin/bash

# Stop the myapp service
sudo systemctl stop openai

# Start the myapp service
sudo systemctl start openai

# Enable the myapp service to start on boot
sudo systemctl enable openai
