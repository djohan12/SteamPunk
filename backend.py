import os
import requests
import json
import sys
import tempfile
from datetime import datetime, timedelta, timezone
from flask import Flask, jsonify, request
from dotenv import load_dotenv 

load_dotenv()
app = Flask(__name__)

CACHE_EXPIRY_HOURS = 1
GAMES_FILE = "games.json"
STEAM_API_KEY = os.getenv("STEAM_API_KEY")

if not STEAM_API_KEY:
    raise RuntimeError("STEAM_API_KEY not found")

def load_data():
    if os.path.exists(GAMES_FILE) and os.path.getsize(GAMES_FILE) > 0:
        with open(GAMES_FILE, "r") as f:
            return json.load(f)
    else:
        return {"accounts":{}}


def save_data(data):
    tmp_file = tempfile.NamedTemporaryFile("w", delete=False, dir=".")
    json.dump(data, tmp_file, indent=4)
    tmp_file.flush()
    os.fsync(tmp_file.fileno())
    tmp_file.close()
    os.replace(tmp_file.name, GAMES_FILE)

def is_cache_fresh(username):
    data = load_data()
    account = data["accounts"].get(username)
    if not account or "last_updated" not in account:
        return False
    last_time = datetime.fromisoformat(account["last_updated"])
    if last_time.tzinfo is None:
        last_time = last_time.replace(tzinfo=timezone.utc)
    return datetime.now(timezone.utc) - last_time < timedelta(hours=CACHE_EXPIRY_HOURS)


def filter_game_data(game):
    appid = game["appid"]
    return {
        "appid": appid,
        "playtime_forever": game["playtime_forever"],
        "img_icon_url": f"http://media.steampowered.com/steamcommunity/public/images/apps/{appid}/{game['img_icon_url']}.jpg",
        "header_url": f"https://steamcdn-a.akamaihd.net/steam/apps/{appid}/header.jpg",
        "store_url": f"https://store.steampowered.com/app/{appid}/"
    }


def get_player_info(steamid):
    url = "https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v2/"
    params = {"key": STEAM_API_KEY, "steamids": steamid}
    response = requests.get(url, params=params, timeout=10)
    response.raise_for_status()
    players = response.json().get("response", {}).get("players", [])
    if not players:
        return None

    player = players[0]
    visibility = player.get("communityvisibilitystate", 0)
    is_public = (visibility == 3)

    return {
        "username": player.get("personaname", "Unknown").replace(" ", "_"),
        "avatar_url": player.get("avatarfull", ""),
        "steamid": steamid,
        "is_public": is_public,
        "visibility": visibility
    }


def retrieve_games(player_info):
    if not player_info or not player_info["is_public"]:
        return None

    url = "https://api.steampowered.com/IPlayerService/GetOwnedGames/v1/"
    params = {
        "key": STEAM_API_KEY,
        "steamid": player_info["steamid"],
        "include_appinfo": True,
        "include_played_free_games": True,
        "format": "json"
    }

    response = requests.get(url, params=params, timeout=10)
    response.raise_for_status()
    games_data = response.json().get("response", {}).get("games", [])
    games_data = sorted(games_data, key=lambda x: x.get("playtime_forever", 0), reverse=True)
    games = {g["name"]: filter_game_data(g) for g in games_data}
    
    data = load_data()
    username = player_info["username"]
    data["accounts"][username] = {
        "steamid": player_info["steamid"],
        "profile_url": f"https://steamcommunity.com/profiles/{player_info['steamid']}",
        "avatar_url": player_info["avatar_url"],
        "games": games,
        "last_updated": datetime.now(timezone.utc).isoformat()
    }
    save_data(data)
    return games

def get_account(username):
    data = load_data()
    account=data["accounts"].get(username)
    if account and is_cache_fresh(username):
        return account
    return None

@app.route("/user/<username>", methods=["GET"])
def get_user(username):
    account = get_account(username)
    if account:
        return jsonify(account)
    
    data = load_data()
    steamid = request.args.get("steamid")
    if not steamid:
        if username in data["accounts"]:
            steamid = data["accounts"][username]["steamid"]
        else:
            return jsonify({"error": "SteamID and username not found"}), 400
    
    player_info = get_player_info(steamid)
    if not player_info or not player_info["is_public"]:
        return jsonify({"error": "Could not retrieve public player info"}), 404
    
    retrieve_games(player_info)
    data = load_data()
    return jsonify(data["accounts"][player_info["username"]])

@app.route("/search", methods=["GET"])
def search_game():
    game_name = request.args.get("game")
    if not game_name:
        return jsonify({"error": "Game not found"}), 400
    
    data = load_data()
    users = []
    img_icon_url = ""
    header_url = ""

    for username, account in data["accounts"].items():
        if game_name in account["games"]:
            game = account["games"][game_name]
            if not img_icon_url:
                img_icon_url = game.get("img_icon_url", "")
            if not header_url:
                header_url = game.get("header_url", "")
            users.append({
                "username": username,
                "profile_url": account["profile_url"],
                "playtime": game["playtime_forever"]
            })

    if not users:
        return jsonify({"error": f"No users found for game '{game_name}'"}), 404

    users.sort(key=lambda x: x["playtime"], reverse=True)

    return jsonify({
        "img_icon_url": img_icon_url,
        "header_url": header_url,
        "users": users
    })


@app.route("/register", methods=["POST"])
def register_user():
    data = load_data()
    req_data = request.get_json(silent=True) or {}

    steamid = req_data.get("steamid")
    if not steamid:
        return jsonify({"error": "SteamID is required"}), 400
    username = req_data.get("username")

    player_info = get_player_info(steamid)
    if not player_info or not player_info["is_public"]:
        return jsonify({"error": "Could not fetch public Steam info"}), 404
    
    if not username:
        username = player_info["username"]

    if username in data["accounts"]:
        return jsonify({"error": "User already registered"}), 400

    data["accounts"][username] = {
        "steamid": steamid,
        "profile_url": f"https://steamcommunity.com/profiles/{steamid}",
        "avatar_url": player_info["avatar_url"],
        "games": {},
        "last_updated": datetime.now(timezone.utc).isoformat()
    }

    save_data(data)
    retrieve_games(player_info)
    account = get_account(username)
    return jsonify(account), 201

    
if __name__ == "__main__":
    app.run(debug=True, host="127.0.0.1", port=5000)

