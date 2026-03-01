# diplom_repo
 diplom_repo_for_golang


# dev notes
1. container health check лег у minio из за отсутствия wget
нужно или костылями делать для прода:
или для мвп руками тестить что все гуд перегуд (localhost и тд)
собственно есть иные образы для minio с доп чеками, но опять же под такой проект это чрезмерно лишние
опять же либо мутить грязь с bash utils и тд
либо до сложжной оркестрации 

    # healthcheck:
    #   test: ["CMD-SHELL", "wget -qO- http://localhost:8222/healthz >/dev/null 2>&1 || exit 1"]
    #   interval: 5s
    #   timeout: 5s
    #   retries: 30

то же с nats, в целом можно использовать tcp чек через nc но это не полная проверка далеко

# пометка:
сделать retry в golang по nats и minio 