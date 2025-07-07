# pterodactyl-backup-cleaner

我使用 pterodactyl panel 管理 Minecraft server，但由於 pterodactyl panel 本身的缺陷 ([`pterodactyl/panel issue #2564`](https://github.com/pterodactyl/panel/issues/2564))，Minecraft server 的 backup 存檔不會在 ，Minecraft server 刪除時自動清除，所以我在不修改 pterodactyl panel 的基礎上，撰寫一個定時清理的服務。  


## 工作流程
定時清理服務使用容器運行，從 pterodactyl 資料庫獲取 backups table 的資料，檢查 backup 存檔目錄下的 `<uuid>.tar.gz` 檔案的 uuid 值是否存在於資料庫索引中，如果 backup 存檔目錄下的檔案的 uuid 值不存在於資料庫中，表示 server 已被刪除但 backup 沒被刪除，可以把該檔案 GC 掉。


## 部署環境
pterodactyl panel 所有 wings 的 backup 存檔使用 NFS 掛載到 TrueNAS 中。  
使用容器部署運行，容器透過 NFS 掛載 pterodactyl 備份目錄







