#include "QmlEnvironmentVariable.h"
#include <stdlib.h>

QString QmlEnvironmentVariable::value(const QString& name)
{
    return qgetenv(qPrintable(name));
}

QString QmlEnvironmentVariable::value(const QString& name, const QString& fallback)
{
    const QString out = qgetenv(qPrintable(name));

    if (out.isEmpty()) {
        return fallback;
    } else {
        return out;
    }
}

void QmlEnvironmentVariable::setValue(const QString& name, const QString &value)
{
    qputenv(qPrintable(name), value.toLocal8Bit());
}

void QmlEnvironmentVariable::unset(const QString& name)
{
    qunsetenv(qPrintable(name));
}

QObject *qmlenvironmentvariable_singletontype_provider(QQmlEngine *, QJSEngine *)
{
    return new QmlEnvironmentVariable();
}