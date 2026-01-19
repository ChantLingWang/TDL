import React from 'react';
import styles from './ArknightsInput.module.scss';
import classNames from 'classnames';

interface ArknightsInputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  label: string;
}

const ArknightsInput: React.FC<ArknightsInputProps> = ({ label, className, ...props }) => {
  return (
    <div className={classNames(styles.inputWrapper, className)}>
      <label className={styles.label}>{label}</label>
      <input className={styles.input} {...props} />
      <div className={styles.decoration} />
    </div>
  );
};

export default ArknightsInput;
