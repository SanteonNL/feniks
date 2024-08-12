SELECT
    concat(p.identificatienummer, humanName.lastname )AS id,
    humanName.lastname as family,
    string_agg(humanName.firstname, '||') AS name
FROM
    patient p
JOIN
    names humanName ON humanName.identificatienummer = p.identificatienummer
GROUP BY
    p.identificatienummer,humanName.lastname;