import React from 'react';
import styles from './ArknightsButton.module.scss';
import classNames from 'classnames';

interface ArknightsButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  label: string;
}

const ArknightsButton: React.FC<ArknightsButtonProps> = ({ label, className, ...props }) => {
  return (
    <button className={classNames(styles.akButton, className)} {...props}>
      {label}
    </button>
  );
};

export default ArknightsButton;
