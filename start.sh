#!/bin/bash
source /opt/discord/SteamPunk/.env

python3 /opt/discord/SteamPunk/backend.py &

/opt/discord/SteamPunk/steampunk

wait

