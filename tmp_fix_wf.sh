#!/bin/bash
mysql -u root -pqweasdzxc123A! starclaw -e "UPDATE workflows SET webhook_token = NULL WHERE webhook_token = '';"
mysql -u root -pqweasdzxc123A! starclaw -e "SELECT COUNT(*) as total FROM workflows;"
